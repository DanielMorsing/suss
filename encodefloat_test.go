package suss

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"

	"testing"
)

func TestEncFloat64(t *testing.T) {
	s := NewTest(t)
	s.Run(func() {
		sign := s.Bool()
		signb := uint64(0)
		if sign {
			signb = uint64(1)
		}
		var fbits uint64
		fbits ^= signb << 63
		exp := drawExponent(s)
		fbits ^= (uint64(exp) << 52)
		mant := s.Uint64() & (^uint64(0) >> (64 - 52))
		fbits ^= mant

		f := math.Float64frombits(fbits)
		b := encodefloat64(f)
		newf, invalid := decodefloat64(b[:])
		newfbits := math.Float64bits(newf)
		if fbits != newfbits || invalid {
			fmt.Printf("orig fbits: %x; new fbits: %x\n", fbits, newfbits)
			s.Fatalf("wrong encoding, orig=%v, new=%v; sign=%v, exp=%v, mant=1.%x", f, newf, signb, exp, mant)
		}
	})
}

func drawExponent(s *Runner) uint16 {
	bits := s.Draw(2, func(r *rand.Rand, n int) []byte {
		var exp uint16
		switch r.Intn(3) {
		case 0:
			exp = 0x7ff
		case 1:
			exp = 0
		case 2:
			exp = uint16(r.Intn(0x7ff))
		}
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], exp)
		return b[:]
	})
	exp := binary.BigEndian.Uint16(bits[:])
	if exp > 0x7ff {
		s.Invalid()
	}
	return exp
}
