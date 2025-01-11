package flex

import (
	"bufio"
	"github.com/idrunk/dce-go/router"
	"github.com/idrunk/dce-go/util"
	"io"
	"math"
	"math/bits"
	"net"
	"slices"
	"sync/atomic"
)

type PackageProtocol[Req any] struct {
	router.Meta[Req]
	pkg *Package
}

func (p *PackageProtocol[Req]) Id() uint32 {
	return p.pkg.Id
}

func (p *PackageProtocol[Req]) Path() string {
	return p.pkg.Path
}

func (p *PackageProtocol[Req]) Sid() string {
	return p.pkg.Sid
}

func (p *PackageProtocol[Req]) Body() ([]byte, error) {
	return p.pkg.parseBody()
}

func (p *PackageProtocol[Req]) ClearBuffer() []byte {
	p.pkg.Sid = p.RespSid()
	p.pkg.Body = p.Meta.ClearBuffer()
	code, message := p.ErrorUnits()
	p.pkg.Code, p.pkg.Message = int32(code), message
	return p.pkg.Serialize()
}

func NewPackageProtocol[Req any](reader *bufio.Reader, req Req, ctxData map[string]any) (*PackageProtocol[Req], error) {
	return NewPackageProtocolWithMeta(reader, router.NewMeta(req, ctxData, true))
}

func NewPackageProtocolWithMeta[Req any](reader *bufio.Reader, meta router.Meta[Req]) (*PackageProtocol[Req], error) {
	pkg, err := PackageDeserializeHead(reader)
	if err != nil {
		return nil, err
	}
	return &PackageProtocol[Req]{meta, pkg}, nil
}

const (
	flagId uint8 = 128 >> iota
	flagPath
	flagSid
	flagCode
	flagMsg
	flagBody
	flagNumPath
)

type Package struct {
	Id      uint32
	Path    string
	NumPath uint32
	Sid     string
	Code    int32
	Message string
	Body    []byte
	bodyLen uint64
	reader  *bufio.Reader
}

func (p *Package) Serialize() []byte {
	// Init a seq buffer and set flag byte to 0
	buffer := make([]byte, 1, 512)
	textBuffer := make([][]byte, 0, 4)
	lenSeqInfoVec := make([]util.Tuple4[byte, int, int, uint], 0, 8)
	// Fill protocol headFlags and pack FlexNum head, cache text contents
	if length := len(p.Path); length > 0 {
		buffer[0] |= flagPath
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(uint16(length))))
		textBuffer = append(textBuffer, []byte(p.Path))
	}
	if length := len(p.Sid); length > 0 {
		buffer[0] |= flagSid
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(uint16(length))))
		textBuffer = append(textBuffer, []byte(p.Sid))
	}
	if length := len(p.Message); length > 0 {
		buffer[0] |= flagMsg
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(uint16(length))))
		textBuffer = append(textBuffer, []byte(p.Message))
	}
	if length := len(p.Body); length > 0 {
		buffer[0] |= flagBody
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(uint(length))))
		textBuffer = append(textBuffer, p.Body)
	}
	if p.Id > 0 {
		buffer[0] |= flagId
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(p.Id)))
	}
	if p.Code != 0 {
		buffer[0] |= flagCode
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(IntPackHead(p.Code)))
	}
	if p.NumPath > 0 {
		buffer[0] |= flagNumPath
		lenSeqInfoVec = append(lenSeqInfoVec, util.NewTuple4(Non0LenPackHead(p.NumPath)))
	}
	// Fill FlexNum heads and pack and fill FlexNum body
	buffer = buffer[:1+len(lenSeqInfoVec)]
	for i, lenSeqInfo := range lenSeqInfoVec {
		buffer[1+i] = lenSeqInfo.A
		buffer = append(buffer, NumPackBody(lenSeqInfo.D, lenSeqInfo.C, lenSeqInfo.B)...)
	}
	// Append the text contents
	for _, part := range textBuffer {
		buffer = append(buffer, part...)
	}
	return buffer
}

func (p *Package) parseBody() ([]byte, error) {
	body := make([]byte, p.bodyLen)
	if _, err := io.ReadFull(p.reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func PackageDeserializeHead(reader *bufio.Reader) (*Package, error) {
	// Try read flag
	flag, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	// Count the FlexNum and init a numHead bytes container
	var numHeadSeq = make([]byte, bits.OnesCount8(flag))
	if _, err = reader.Read(numHeadSeq); err != nil {
		return nil, err
	}
	var numInfoSeq = make([]util.Tuple5[byte, uint8, bool, byte, []byte], len(numHeadSeq))
	// Parse the FlexNum head and total the FlexNum body len
	for i, numHead := range numHeadSeq {
		unsignedBits, bytesLen, negative, originalBits := NumParseHead(numHead, true)
		// Read FlexNum bodies
		var numBodySeq = make([]byte, bytesLen)
		if _, err = reader.Read(numBodySeq); err != nil {
			return nil, err
		}
		numInfoSeq[i] = util.NewTuple5(unsignedBits, bytesLen, negative, originalBits, numBodySeq)
	}
	pkg := Package{reader: reader}
	// Finally number parse and read text seq
	if flag&flagPath > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		seq := make([]byte, Non0LenParse(numInfo[0].D, numInfo[0].E))
		if _, err = io.ReadFull(reader, seq); err != nil {
			return nil, err
		}
		pkg.Path = string(seq)
	}
	if flag&flagSid > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		seq := make([]byte, Non0LenParse(numInfo[0].D, numInfo[0].E))
		if _, err = io.ReadFull(reader, seq); err != nil {
			return nil, err
		}
		pkg.Sid = string(seq)
	}
	if flag&flagMsg > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		seq := make([]byte, Non0LenParse(numInfo[0].D, numInfo[0].E))
		if _, err = io.ReadFull(reader, seq); err != nil {
			return nil, err
		}
		pkg.Message = string(seq)
	}
	if flag&flagBody > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		pkg.bodyLen = Non0LenParse(numInfo[0].D, numInfo[0].E)
	}
	if flag&flagId > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		pkg.Id = uint32(Non0LenParse(numInfo[0].D, numInfo[0].E))
	}
	if flag&flagCode > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		pkg.Code = IntParse[int32](numInfo[0].C, numInfo[0].A, numInfo[0].E)
	}
	if flag&flagNumPath > 0 {
		numInfo, _ := util.SliceDeleteGet(&numInfoSeq, 0, 1)
		pkg.NumPath = uint32(Non0LenParse(numInfo[0].D, numInfo[0].E))
	}
	return &pkg, nil
}

func PackageDeserialize(reader *bufio.Reader) (*Package, error) {
	sp, err := PackageDeserializeHead(reader)
	if err != nil {
		return nil, err
	} else if b, e := sp.parseBody(); e != nil {
		return nil, e
	} else {
		sp.Body = b
		return sp, nil
	}
}

var reqId atomic.Uint32

func NewPackage(path string, body []byte, sid string, id int) *Package {
	return NewNumPackage(0, body, sid, id, path)
}

func NewNumPackage(numPath uint32, body []byte, sid string, id int, path string) *Package {
	if id == -1 {
		reqId.Add(1)
		if id = int(reqId.Load()); id == math.MaxUint32 {
			reqId.Store(0)
		}
	}
	return &Package{Id: uint32(id), Path: path, NumPath: numPath, Sid: sid, Body: body}
}

func UintSerialize[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) []byte {
	return numSerialize(UintPackHead(unsigned))
}

func IntSerialize[I int | int64 | int32 | int16 | int8](integer I) []byte {
	return numSerialize(IntPackHead(integer))
}

// Non0LenPackHead can package 128 into uint7 to represent the length of sha512 in hexadecimal
func Non0LenPackHead[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) (head byte, bytesLen int, bitsLen int, usize uint) {
	return UintPackHead(unsigned - 1)
}

func UintPackHead[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) (head byte, bytesLen int, bitsLen int, usize uint) {
	usize = uint(unsigned)
	bitsLen = bits.Len(usize)
	head, bytesLen = numPackHead(usize, bitsLen)
	return head, bytesLen, bitsLen, usize
}

func IntPackHead[I int | int64 | int32 | int16 | int8](integer I) (byte, int, int, uint) {
	var unsigned uint
	if integer < 0 {
		// use -int to resolve the edge case, eg. int8(-128) to int8(128) is illegal, but to int(128) is legal.
		// but the int(math.MinInt) to -int(math.MinInt) may still illegal, but it is unlikely to be used.
		unsigned = uint(-int(integer))
	} else {
		unsigned = uint(integer)
	}
	// add the width of the sign
	bitsLen := bits.Len(unsigned) + 1
	head, bytesLen := numPackHead(unsigned, bitsLen)
	if integer < 0 {
		var negative uint8 = 1
		if bytesLen < 7 {
			negative = 1 << (6 - bytesLen)
		}
		// handle the negative situation to mark the sign bit
		head |= negative
	}
	return head, bytesLen, bitsLen, unsigned
}

func numPackHead[U uint | uint64 | uint32 | uint16 | uint8](u64 U, bitsLen int) (byte, int) {
	bytesLen := int(math.Floor(float64(bitsLen) / 8))
	headMaskShift := 8 - bytesLen
	var headBits uint8
	if bytesLen > 5 {
		// Directly alloc 8bytes if the requirement greater than 5
		bytesLen = 8
		headMaskShift = 2
	} else if bitsLen%8 > 7-bytesLen {
		// If the remains bits width than the headBits, then need to increase the bytes length,
		// and decrease the maskShift to match the body byte length.
		// When bytesLen updated, the head no longer needs to be stored any bits, just need to keep the default
		bytesLen++
		headMaskShift--
	} else {
		// Otherwise, right shift the bodyBytesLen to calc out the headBits
		headBits |= uint8(u64 >> (bytesLen * 8))
	}
	return 255<<headMaskShift&255 | headBits, bytesLen
}

func numSerialize[U uint | uint64 | uint32 | uint16 | uint8](head uint8, bytesLen int, bitsLen int, u64 U) []byte {
	units := make([]byte, bytesLen+1)
	units[0] = head
	copy(units[1:], NumPackBody(u64, bitsLen, bytesLen))
	return units
}

func NumPackBody[U uint | uint64 | uint32 | uint16 | uint8](u64 U, bitsLen int, bytesLen int) []byte {
	units := make([]byte, bytesLen)
	for i := 0; i < bytesLen && i*8 < bitsLen; i++ {
		units[bytesLen-i-1] = uint8(u64 >> (i * 8) & 255)
	}
	return units
}

func UintDeserialize[U uint | uint64 | uint32 | uint16 | uint8](seq []byte) U {
	seq[0], _, _, _ = NumParseHead(seq[0], false)
	return U(NumParse(seq))
}

func IntDeserialize[I int | int64 | int32 | int16 | int8](seq []byte) I {
	headBits, _, negative, _ := NumParseHead(seq[0], true)
	return IntParse[I](negative, headBits, seq[1:])
}

func NumParseHead(head byte, sign bool) (unsignedBits byte, bytesLen uint8, negative bool, originalBits byte) {
	for i := 0; i < 8; i++ {
		if 128>>i&head == 0 {
			if bytesLen = uint8(i); bytesLen > 5 {
				bytesLen = 8
				originalBits = 1 & head
			} else {
				originalBits = 127 >> bytesLen & head
			}
			break
		}
	}
	unsignedBits = originalBits
	if sign {
		if bytesLen == 8 {
			negative = 1&head == 1
		} else {
			signShift := 0
			if negative = 64>>bytesLen&head > 0; negative {
				signShift = 1
			}
			unsignedBits = 127 >> bytesLen >> signShift & head
		}
	}
	return unsignedBits, bytesLen, negative, originalBits
}

func IntParse[I int | int64 | int32 | int16 | int8](negative bool, head uint8, seq []byte) I {
	u64 := NumParse(slices.Insert(seq, 0, head))
	if negative {
		return I(-int64(u64))
	}
	return I(u64)
}

func Non0LenParse(head uint8, seq []byte) uint64 {
	return NumParse(slices.Insert(seq, 0, head)) + 1
}

func NumParse(seq []byte) uint64 {
	var u64 uint64 = 0
	for i, b := range seq {
		if b > 0 {
			u64 |= uint64(b) << ((len(seq) - i - 1) * 8)
		}
	}
	return u64
}

func StreamRead(conn net.Conn) ([]byte, error) {
	reader := bufio.NewReader(conn)
	numHead, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	_, bytesLen, _, headBits := NumParseHead(numHead, false)
	numBody := make([]byte, bytesLen)
	if _, err = reader.Read(numBody); err != nil {
		return nil, err
	}
	var data = make([]byte, Non0LenParse(headBits, numBody))
	if _, err = io.ReadFull(reader, data); err != nil {
		return data, err
	}
	return data, err
}

func StreamPack(bytes []byte) []byte {
	head, bytesLen, bitsLen, usize := Non0LenPackHead(uint(len(bytes)))
	return slices.Insert(bytes, 0, slices.Insert(NumPackBody(usize, bitsLen, bytesLen), 0, head)...)
}
