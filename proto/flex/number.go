package flex

import (
	"math"
	"math/bits"
	"slices"
)

type NumHead struct {
	Unsigned byte   // 整数除开符号位的头部
	BytesLen uint8  // 数字躯体部分字节长度
	Negative bool   // 是否负数
	Original byte   // 整数头部（若为有符号整数，将符号位也视为数字一部分）
	Head     byte   // 弹性数字序列头
	BitsLen  uint8  // 数字比特长度
	Positive uint64 // 正整数
}

func UintSerialize[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) []byte {
	return numSerialize(UintPackHead(unsigned))
}

func IntSerialize[I int | int64 | int32 | int16 | int8](integer I) []byte {
	return numSerialize(IntPackHead(integer))
}

func numSerialize(nh *NumHead) []byte {
	units := make([]byte, nh.BytesLen+1)
	units[0] = nh.Head
	copy(units[1:], NumPackBody(nh))
	return units
}

func NumPackBody(nh *NumHead) []byte {
	units := make([]byte, nh.BytesLen)
	for i := uint8(0); i < nh.BytesLen && i*8 < nh.BitsLen; i++ {
		units[nh.BytesLen-i-1] = uint8(int(nh.Positive) >> (i * 8) & 255)
	}
	return units
}

// Non0LenPackHead can package 128 into uint7 to represent the length of sha512 in hexadecimal
func Non0LenPackHead[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) *NumHead {
	return UintPackHead(unsigned - 1)
}

func UintPackHead[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) *NumHead {
	nh := &NumHead{}
	nh.Positive = uint64(unsigned)
	nh.BitsLen = uint8(bits.Len64(nh.Positive))
	nh.Head, nh.BytesLen = numPackHead(nh.Positive, nh.BitsLen)
	return nh
}

func IntPackHead[I int | int64 | int32 | int16 | int8](integer I) *NumHead {
	nh := &NumHead{}
	if integer < 0 {
		// use -int to resolve the edge case, eg. int8(-128) to int8(128) is illegal, but to int(128) is legal.
		// but the int(math.MinInt) to -int(math.MinInt) may still illegal, but it is unlikely to be used.
		nh.Positive = uint64(-int64(integer))
	} else {
		nh.Positive = uint64(integer)
	}
	// add the width of the sign
	nh.BitsLen = uint8(bits.Len64(nh.Positive)) + 1
	nh.Head, nh.BytesLen = numPackHead(nh.Positive, nh.BitsLen)
	if integer < 0 {
		var negative uint8 = 1
		if nh.BytesLen < 7 {
			negative = 1 << (6 - nh.BytesLen)
		}
		// handle the negative situation to mark the sign bit
		nh.Head |= negative
	}
	return nh
}

func numPackHead(u64 uint64, bitsLen uint8) (byte, uint8) {
	bytesLen := uint8(math.Floor(float64(bitsLen) / 8))
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

func UintDeserialize[U uint | uint64 | uint32 | uint16 | uint8](seq []byte) U {
	nh := NumParseHead(seq[0], false)
	seq[0] = nh.Unsigned
	return U(NumParse(seq))
}

func IntDeserialize[I int | int64 | int32 | int16 | int8](seq []byte) I {
	nh := NumParseHead(seq[0], true)
	return I(IntParse(nh.Negative, nh.Unsigned, seq[1:]))
}

func NumParseHead(head byte, sign bool) *NumHead {
	nh := &NumHead{}
	for i := range 8 {
		if 128>>i&head == 0 {
			if nh.BytesLen = uint8(i); nh.BytesLen > 5 {
				nh.BytesLen = 8
				nh.Original = 1 & head
			} else {
				nh.Original = 127 >> nh.BytesLen & head
			}
			break
		}
	}
	nh.Unsigned = nh.Original
	if sign {
		if nh.BytesLen == 8 {
			nh.Negative = 1&head == 1
		} else {
			signShift := 0
			if nh.Negative = 64>>nh.BytesLen&head > 0; nh.Negative {
				signShift = 1
			}
			nh.Unsigned = 127 >> nh.BytesLen >> signShift & head
		}
	}
	return nh
}

func IntParse(negative bool, head uint8, seq []byte) int64 {
	i64 := int64(NumParse(slices.Insert(seq, 0, head)))
	if negative {
		i64 = -i64
	}
	return i64
}

func UintParse(head uint8, seq []byte) uint64 {
	return NumParse(slices.Insert(seq, 0, head))
}

func Non0LenParse(head uint8, seq []byte) uint64 {
	return UintParse(head, seq) + 1
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
