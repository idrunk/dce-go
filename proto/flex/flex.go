package flex

import (
	"bufio"
	"io"
	"log"
	"math"
	"math/bits"
	"net"
	"reflect"
	"slices"
	"sync/atomic"
	"unsafe"

	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/util"
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

type PackageField struct {
	Field string
	Kind reflect.Kind
	Get func(fc *PackageField, pkg *reflect.Value) (numHead *NumHead, textSeq []byte)
	Set func(fc *PackageField, pkg *reflect.Value, nh *NumHead, nbSeq []byte, reader io.Reader) (err error)
}

func DefaultPropertyGetter(fc *PackageField, pkg *reflect.Value) (numHead *NumHead, textSeq []byte) {
	val := pkg.FieldByName(fc.Field)
	if val.IsZero() {
		// if (field.Required) {
		// 	log.Panicf("%s is required in Package.", field.Field)
		// }
		return
	}

	if fc.Kind >= reflect.Int && fc.Kind < reflect.Uint { // int
		numHead = IntPackHead(val.Int())
	} else if fc.Kind >= reflect.Uint && fc.Kind <= reflect.Uint64 { // uint
		numHead = UintPackHead(val.Uint())
	} else if fc.Kind == reflect.String { // string
		str := val.String()
		numHead = Non0LenPackHead(uint(len(str)))
		textSeq = []byte(str)
	} else if bts, ok := val.Interface().([]byte); ok { // []byte
		numHead = Non0LenPackHead(uint(len(bts)))
		textSeq = bts
	}
	return
}

func DefaultPropertySetter(fc *PackageField, pkg *reflect.Value, nh *NumHead, nbSeq []byte, reader io.Reader) (err error) {
	tyRef := pkg.Type()
	f, ok := tyRef.FieldByName(fc.Field)
	if !ok {
		log.Fatalf(`Field "%s" not defined in Package`, fc.Field)
	}
	field := pkg.FieldByIndex(f.Index)
	switch fc.Kind {
	case reflect.String, reflect.Slice:
		len := Non0LenParse(nh.Original, nbSeq)
		if fc.Kind == reflect.String {
			seq := make([]byte, len)
			if _, err = io.ReadFull(reader, seq); err != nil {
				return err
			}
			field.SetString(string(seq))
		} else if fc.Field == "Body" { // Hard-coding for `Body`
			blf := pkg.FieldByName("bodyLen")
			ptr := unsafe.Pointer(blf.UnsafeAddr())
			realPtr := (*uint64)(ptr)
			*realPtr = len
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val := IntParse(nh.Negative, nh.Unsigned, nbSeq)
		field.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val := UintParse(nh.Original, nbSeq)
		field.SetUint(val)
	}
	return nil
}

var baseFields = []*PackageField{
	{"Id", reflect.Uint32, DefaultPropertyGetter, DefaultPropertySetter},
	{"Path", reflect.String, DefaultPropertyGetter, DefaultPropertySetter},
	{"NumPath", reflect.Uint32, DefaultPropertyGetter, DefaultPropertySetter},
	{"Sid", reflect.String, DefaultPropertyGetter, DefaultPropertySetter},
	{"Code", reflect.Int32, DefaultPropertyGetter, DefaultPropertySetter},
	{"Message", reflect.String, DefaultPropertyGetter, DefaultPropertySetter},
	{"Body", reflect.Slice, DefaultPropertyGetter, DefaultPropertySetter},
}

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
	return p.SerializeWith(nil, nil)
}

func (p *Package) mergeFields(fields []*PackageField, pkg *reflect.Value) []*util.Tuple2[*PackageField, *reflect.Value] {
	pe := reflect.ValueOf(p).Elem()
	fullFields := make([]*util.Tuple2[*PackageField, *reflect.Value], 0, len(baseFields))
	for _, f := range baseFields {
		fullFields = append(fullFields, util.NewTuple2(f, &pe))
	}
	if fields != nil && pkg != nil {
		for _, f := range fields {
			fullFields = append(fullFields, util.NewTuple2(f, pkg))
		}
	}
	return fullFields
}

func (p *Package) SerializeWith(fields []*PackageField, pkg *reflect.Value) []byte {
	fullFields := p.mergeFields(fields, pkg)
	flag := 0
	totalFields := len(fullFields)
	numHeadVec := make([]*NumHead, 0, totalFields + 1)
	textBuffer := make([][]byte, 0, totalFields - 4) // 容量为减掉至少4个非文本的字段数
	bodyBuffer := make([][]byte, 0, 1)
	for i, t2 := range fullFields {
		numHead, textSeq := t2.A.Get(t2.A, t2.B);
		if numHead == nil {
			continue
		}
		flag |= 1 << i
		numHeadVec = append(numHeadVec, numHead)
		if t2.A.Kind == reflect.Slice {
			bodyBuffer = append(bodyBuffer, textSeq)
		} else if len(textSeq) > 0 {
			textBuffer = append(textBuffer, textSeq)
		}
	}

	flagSeq := UintSerialize(uint(flag))
	flagLen := len(flagSeq)
	buffer := make([]byte, flagLen, 512)
	copy(buffer, flagSeq)
	// Fill FlexNum heads and pack and fill FlexNum body
	buffer = buffer[:flagLen+len(numHeadVec)]
	for i, nh := range numHeadVec {
		buffer[flagLen+i] = nh.Head
		buffer = append(buffer, NumPackBody(nh)...)
	}
	// Append the text contents
	for _, part := range textBuffer {
		buffer = append(buffer, part...)
	}
	for _, part := range bodyBuffer {
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

func PackageDeserializeHead(reader *bufio.Reader) (*Package, error) {
	return PackageDeserializeHeadWith(reader, nil, nil)
}

func PackageDeserializeHeadWith(reader *bufio.Reader, fields []*PackageField, pkg *reflect.Value) (*Package, error) {
	// Try read flagHead
	head, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	nh := NumParseHead(head, false)
	flag := uint64(nh.Original)
	if nh.BytesLen > 0 {
		var flagBodySeq = make([]byte, nh.BytesLen)
		if _, err = reader.Read(flagBodySeq); err != nil {
			return nil, err
		}
		flagBodySeq = slices.Insert(flagBodySeq, 0, nh.Original)
		flag = NumParse(flagBodySeq)
	}

	// Count the FlexNum and init a numHead bytes container
	headOnesCount := bits.OnesCount64(flag)
	numHeadList := make([]byte, headOnesCount)
	if _, err = reader.Read(numHeadList); err != nil {
		return nil, err
	}
	numInfoList := make([]*util.Tuple3[int, *NumHead, []byte], headOnesCount)
	nhi := 0
	bitsLen := bits.Len64(flag)
	for i := range bitsLen {
		if 1 << i & flag == 0 {
			continue // Directly continue if current bit is zero
		}
		nh := NumParseHead(numHeadList[nhi], true)
		// Read FlexNum bodies
		var numBodySeq = make([]byte, nh.BytesLen)
		if _, err = reader.Read(numBodySeq); err != nil {
			return nil, err
		}
		numInfoList[nhi] = util.NewTuple3(i, nh, numBodySeq)
		nhi ++
	}

	p := &Package{reader: reader}
	pkgRef := reflect.ValueOf(p).Elem()
	fullFields := p.mergeFields(fields, &pkgRef)
	if bitsLen > len(fullFields) {
		return nil, util.Closed0(`Packet exception, flag overflow`)
	}
	for _, ni := range numInfoList {
		fc := fullFields[ni.A]
		fc.A.Set(fc.A, fc.B, ni.B, ni.C, reader)
	}
	return p, nil
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

func StreamRead(conn net.Conn) ([]byte, error) {
	reader := bufio.NewReader(conn)
	numHead, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	nh := NumParseHead(numHead, false)
	numBody := make([]byte, nh.BytesLen)
	if _, err = reader.Read(numBody); err != nil {
		return nil, err
	}
	var data = make([]byte, Non0LenParse(nh.Head, numBody))
	if _, err = io.ReadFull(reader, data); err != nil {
		return data, err
	}
	return data, err
}

func StreamPack(bytes []byte) []byte {
	nh := Non0LenPackHead(uint(len(bytes)))
	return slices.Insert(bytes, 0, slices.Insert(NumPackBody(nh), 0, nh.Head)...)
}
