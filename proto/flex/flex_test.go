package flex

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"math"
	"math/bits"
	"math/rand/v2"
	"strings"
	"testing"
)

func TestPackage_Serialize(t *testing.T) {
	body := []byte(strings.Repeat("Hello world!你好，世界！", rand.IntN(1000)))
	hash := md5.New()
	hash.Write(body)
	srcHash := hex.EncodeToString(hash.Sum(nil))
	t.Logf("Source content hash: %s", srcHash)
	pkg := NewPackage("home", body, srcHash, -1)
	seq := pkg.Serialize()
	dePkg, _ := PackageDeserialize(bufio.NewReader(bytes.NewReader(seq)))
	hash2 := md5.New()
	hash2.Write(dePkg.Body)
	handledHash := hex.EncodeToString(hash2.Sum(nil))
	t.Logf("Handled content hash: %s", handledHash)
	if srcHash != handledHash {
		t.Fail()
	}
}

func TestFlexNumSerialize(t *testing.T) {
	for _, n := range []any{
		uint8(127),
		uint8(128),
		uint8(129),
		uint8(255),
		uint16(256),
		uint16(257),
		uint16(16383),
		uint16(16384),
		uint16(16385),
		uint64(999999999999999999),

		int8(63),
		int8(64),
		int8(65),
		int8(127),
		int16(128),
		int16(129),
		int16(16383),
		int16(16384),
		int16(16385),
		int64(999999999999999999),

		int8(-63),
		int8(-64),
		int8(-65),
		int8(-127),
		int8(-128),
		int16(-129),
		int16(-16383),
		int16(-16384),
		int16(-16385),
		int64(-999999999999999999),
	} {
		testFlexNumSerialize(t, n)
	}
}

func testFlexNumSerialize(t *testing.T, num any) {
	var val any
	switch n := num.(type) {
	case uint:
		val = UintDeserialize[uint](UintSerialize(n))
	case uint64:
		val = UintDeserialize[uint64](UintSerialize(n))
	case uint32:
		val = UintDeserialize[uint32](UintSerialize(n))
	case uint16:
		val = UintDeserialize[uint16](UintSerialize(n))
	case uint8:
		val = UintDeserialize[uint8](UintSerialize(n))
	case int:
		val = IntDeserialize[int](IntSerialize(n))
	case int64:
		val = IntDeserialize[int64](IntSerialize(n))
	case int32:
		val = IntDeserialize[int32](IntSerialize(n))
	case int16:
		val = IntDeserialize[int16](IntSerialize(n))
	case int8:
		val = IntDeserialize[int8](IntSerialize(n))
	}
	if val != num {
		t.Errorf("Deserialized num \"%d\" was not eq to \"%d\".", val, num)
	}
}

func TestBits7NumSerialize(t *testing.T) {
	for _, n := range []any{
		uint8(127),
		uint8(128),
		uint8(129),
		uint8(255),
		uint16(256),
		uint16(257),
		uint16(16383),
		uint16(16384),
		uint16(16385),
		uint64(999999999999999999),

		int8(63),
		int8(64),
		int8(65),
		int8(127),
		int16(128),
		int16(129),
		int16(16383),
		int16(16384),
		int16(16385),
		int64(99999999999999),

		int8(-63),
		int8(-64),
		int8(-65),
		int8(-127),
		int8(-128),
		int16(-129),
		int16(-16383),
		int16(-16384),
		int16(-16385),
		int64(-99999999999999),
	} {
		testBits7NumSerialize(t, n)
	}
}

func testBits7NumSerialize(t *testing.T, num any) {
	var val any
	switch n := num.(type) {
	case uint:
		val = Bits7UintDeserialize[uint](Bits7UintSerialize(n))
	case uint64:
		val = Bits7UintDeserialize[uint64](Bits7UintSerialize(n))
	case uint32:
		val = Bits7UintDeserialize[uint32](Bits7UintSerialize(n))
	case uint16:
		val = Bits7UintDeserialize[uint16](Bits7UintSerialize(n))
	case uint8:
		val = Bits7UintDeserialize[uint8](Bits7UintSerialize(n))
	case int:
		val = Bits7IntDeserialize[int](Bits7IntSerialize(n))
	case int64:
		val = Bits7IntDeserialize[int64](Bits7IntSerialize(n))
	case int32:
		val = Bits7IntDeserialize[int32](Bits7IntSerialize(n))
	case int16:
		val = Bits7IntDeserialize[int16](Bits7IntSerialize(n))
	case int8:
		val = Bits7IntDeserialize[int8](Bits7IntSerialize(n))
	}
	if val != num {
		t.Errorf("Deserialized num \"%d\" was not eq to \"%d\".", num, val)
	}
}

func Bits7UintDeserialize[U uint | uint64 | uint32 | uint16 | uint8](seq []byte) U {
	return U(bits7NumDeserialize(seq, 0))
}

func Bits7IntDeserialize[I int | int64 | int32 | int16 | int8](seq []byte) I {
	u64 := bits7NumDeserialize(seq, 1)
	var negativeSign uint8 = 64
	if len(seq) == 9 {
		negativeSign = 128
	}
	if seq[len(seq)-1]&negativeSign != 0 {
		return I(-u64)
	}
	return I(u64)
}

func bits7NumDeserialize(seq []byte, highMarkShift int) uint64 {
	var u64 uint64 = 0
	for i, lastIndex := 0, len(seq)-1; i <= lastIndex; i++ {
		if i != lastIndex {
			u64 |= uint64(seq[i]&127) << (i * 7)
		} else if i == 8 {
			u64 |= uint64(seq[i]&(255>>highMarkShift)) << (i * 7)
		} else {
			u64 |= uint64(seq[i]&(127>>highMarkShift)) << (i * 7)
		}
	}
	return u64
}

func Bits7UintSerialize[U uint | uint64 | uint32 | uint16 | uint8](unsigned U) []byte {
	return bits7NumSerialize(unsigned, bits.Len(uint(unsigned)))
}

func Bits7IntSerialize[I int | int64 | int32 | int16 | int8](integer I) []byte {
	var mask uint8 = 0
	u64 := uint64(integer)
	if integer < 0 {
		mask = 64
		u64 = uint64(-integer)
	}
	bit7Units := bits7NumSerialize(u64, bits.Len(uint(u64))+1)
	if mask > 0 {
		lastIndex := len(bit7Units) - 1
		if lastIndex == 8 {
			mask = 128
		}
		bit7Units[lastIndex] |= mask
	}
	return bit7Units
}

func bits7NumSerialize[U uint | uint64 | uint32 | uint16 | uint8](u64 U, bitsLen int) []byte {
	bit7Units := make([]byte, int(math.Ceil(float64(bitsLen)/7)))
	lastIndex := len(bit7Units) - 1
	for i := 0; i <= lastIndex; i++ {
		if i != lastIndex {
			bit7Units[i] = uint8(u64>>(7*i)&127) | 128
		} else if i == 8 {
			bit7Units[i] = uint8(u64 >> (7 * i) & 255)
		} else {
			bit7Units[i] = uint8(u64 >> (7 * i) & 127)
		}
	}
	return bit7Units
}
