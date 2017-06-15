package suss

import (
	"encoding/binary"
	"math"
	"math/rand"
)

type SliceGen struct {
	Avg int
	Min int
	Max int

	g *Generator
}

func (g *Generator) Slice() *SliceGen {
	return &SliceGen{
		Avg: 50,
		Min: 0,
		Max: int(^uint(0) >> 1),
		g:   g,
	}
}

func (s *SliceGen) Gen(f func()) {
	// The intuitive way to turn an infinite bytestream into a
	// slice would be to grab a value at the beginning
	// and then generate that number of elements
	// However, this gets in the way of shrinking
	//
	// Instead, for each element, grab a byte
	// asking us if we want more elements.
	// That way, deleting a span in the byte
	// stream turns into the element not being
	// added.
	l := uint64(0)
	stopvalue := 1 - (1.0 / (1 + float64(s.Avg)))
	if s.Min < 0 {
		panic("invalid min slice length")
	}
	min := uint64(s.Min)
	max := uint64(s.Max)
	for l < max {
		s.g.StartExample()
		more := s.g.biasBool(stopvalue)
		if !more && l >= min {
			s.g.EndExample()
			return
		}
		l++
		f()
		s.g.EndExample()
	}
}

func (g *Generator) Bool() bool {
	b := g.Draw(1, Uniform)
	return b[0]&1 == 1
}

func (g *Generator) Float64() float64 {
	g.StartExample()
	fbits := g.Draw(10, func(r *rand.Rand, n int) []byte {
		if n != 10 {
			panic("bad float size")
		}
		flavor := r.Intn(5)
		var f float64
		switch flavor {
		case 0:
			f = math.NaN()
		case 1:
			f = math.Inf(0)
		case 2:
			f = math.Inf(-1)
		case 3:
			// TODO incorporate evil floats from hypothesis
			f = 0
		default:
			f = r.Float64()
			f *= math.MaxFloat64
			if r.Intn(2) == 1 {
				f = -f
			}
		}
		b := encodefloat64(f)
		return b[:]
	})
	g.EndExample()
	f, invalid := decodefloat64(fbits)
	if invalid {
		g.Invalid()
	}
	return f
}

// encodefloat64 attempts to encode a floating point number
// so that its lexicographical ordering follows human intuition
//
// Design goals were:
// - Integers are simpler than fractionals
// - positive numbers are simpler than negative ones
// - exponents of smaller magnitude are simpler, regardless of sign
// - 0 is the simplest number, 1 is the second most simple number
func encodefloat64(f float64) [10]byte {
	var b [10]byte
	bits := math.Float64bits(f)
	// encode the sign bit as a single byte
	b[0] = byte((bits & (1 << 63)) >> 63)

	// for the mantissa, we want simpler fractions
	// This means we get numbers that require fewer
	// digits to print it
	// Encoding as a little endian number
	// makes shrinking go towards a number with
	// fewer significant digits
	mant := bits & (^uint64(0) >> (64 - 52))
	binary.LittleEndian.PutUint64(b[1:], mant)

	// if the exponent is 0, that means this value
	// is a zero. don't unbias the exponent in this case
	// While we should be able to just fill the rest of the buffer with
	// zeros, we might have got a weird encoding of 0 that could cause
	// a fault
	sexp := int16((bits >> 52) & 0x7ff)
	var exp uint16
	if sexp != 0 {
		sexp -= 1023
		// if exponent is positive, bias it +1
		// so that an exponent of 1 becomes 0
		// This keeps the invariant that 0 is
		// simpler than 1
		if sexp >= 0 {
			exp = uint16(sexp) + 1
		} else {
			// for negative exponents
			// use signed regular integer
			// This makes -1 simpler than -2
			// when interpreted as a byte stream
			// the sign keeps the invariant that
			// integers are simpler than fractionals
			sexp *= -1
			exp = uint16(sexp)
			exp ^= (1 << 15)
		}
	}
	binary.BigEndian.PutUint16(b[8:], exp)
	return b
}

func decodefloat64(b []byte) (float64, bool) {
	fbits := uint64(0)
	sign := b[0]
	if sign != 0 && sign != 1 {
		return 0, true
	}
	fbits = uint64(sign) << 63
	exp := binary.BigEndian.Uint16(b[8:])
	if exp&(1<<15) != 0 {
		// this is a signed exponent
		// clear the sign bit
		sexp := int16(exp & (^uint16(0) >> 1))
		// make into negative number
		sexp *= -1
		// unbias
		sexp += 1023
		exp = uint16(sexp)
	} else if exp != 0 {
		// positive exponent
		exp -= 1
		exp += 1023
	}
	fbits ^= uint64(exp) << 52
	// mantissa only take 7 bytes in our binary packing
	// but binary only lets us read in chunks of 8
	// copy the mantissa value into an empty array
	// and then decode to make sure that we don't
	var mb [8]byte
	copy(mb[:], b[1:8])
	mant := binary.LittleEndian.Uint64(mb[:])
	fbits ^= mant & (^uint64(0) >> (64 - 52))
	return math.Float64frombits(fbits), false
}

func (g *Generator) Uint64() uint64 {
	g.StartExample()
	f := g.Draw(8, Uniform)
	g.EndExample()
	return binary.BigEndian.Uint64(f)
}

func (g *Generator) Int16() int16 {
	g.StartExample()
	f := g.Draw(8, Uniform)
	g.EndExample()
	return int16(binary.BigEndian.Uint16(f))
}

func (g *Generator) Byte() byte {
	g.StartExample()
	defer g.EndExample()
	return g.Draw(1, Uniform)[0]
}

func (g *Generator) biasBool(f float64) bool {
	bits := g.Draw(1, func(r *rand.Rand, n int) []byte {
		roll := r.Float64()
		b := byte(0)
		if roll < f {
			b = 1
		}
		return []byte{b}
	})
	return bits[0] != 0
}
