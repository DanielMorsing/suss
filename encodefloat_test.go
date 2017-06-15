package suss

import (
	"fmt"
	"math"

	"testing"
)

func TestEncFloat64(t *testing.T) {
	s := NewTest(t)
	s.Run(func() {
		sign := s.Bool()
		exp := s.Int16()
		if exp > 1024 || exp < -1023 {
			s.Invalid()
		}
		signb := uint64(0)
		if sign {
			signb = uint64(1)
		}
		var fbits uint64
		fbits ^= signb << 63
		// unbias the exponent
		bexp := exp + 1023
		fbits ^= (uint64(bexp) << 52)
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
