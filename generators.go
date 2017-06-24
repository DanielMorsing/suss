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

	f func()
}

// Generator generates
// data to be used in a test.
// Fill should draw bytes from the Data interface
// and change its own value based off those bytes.
//
// Generators can be passed to the Runner.Draw function
// to be supplied with a Data to draw bytes from.
// This is the main way to get varying data that might
// cause tests to fail.
type Generator interface {
	Fill(d Data)
}

// Data is an interface passed to Fill methods on types implementing
// the Generator interface. It is the main method for getting random
// data from the Suspicion runner.
type Data interface {
	// Draw is the main way to get random data
	// for a Suspicion test. It takes a number of bytes
	// to draw and a Sample function. The Sample function
	// should return a valid byte sequence for the value.
	//
	// Since return values from Sample is used as examples
	// during execution, it is adventageous to return
	// interesting values. Floating point Not-a-Number
	// and Infinity values is a good example of such a
	// value.
	//
	// During test execution, Draw might return the value
	// from Sample or a random one. Callers must handle
	// any random byte sequence, either by reinterpreting it
	// or calling Invalid.
	Draw(n int, smp Sample) []byte

	// StartExample and EndExample are used to specify boundaries for
	// draws. The shrinking algorithm uses these boundaries to make
	// decisions about how to shrink.
	//
	// Most users will not need to
	// explicitly call these functions, since Draw inserts StartExample
	// and EndExample calls around calls to Fill.
	//
	// Calls to these functions can be nested.
	StartExample()
	EndExample()
}

// Slice returns a generator for a slice value.
// It will repeatedly call the given function
// during fill. It is the functions responsibility
// to build the slice wanted.
//
// An example of the proper use of
// Slice:
//
//	var f []float64
//	s := suss.Slice(func() {
//		f = append(f, runner.Float64())
//	})
//	runner.Draw(s)
func Slice(f func()) *SliceGen {
	return &SliceGen{
		Avg: 50,
		Min: 0,
		Max: int(^uint(0) >> 1),
		f:   f,
	}
}

func (s *SliceGen) Fill(d Data) {
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
		d.StartExample()
		more := biasBool(d, stopvalue)
		if !more && l >= min {
			d.EndExample()
			return
		}
		l++
		sliceCall(s.f, d)
	}
}

// Since slice functions can panic, make sure that we
// always call d.EndExample
func sliceCall(f func(), d Data) {
	defer d.EndExample()
	f()
}

// BoolGen implements a generator for boolean values.
type BoolGen bool

func (b *BoolGen) Fill(d Data) {
	byt := d.Draw(1, Uniform)
	*b = byt[0]&1 == 1
}

// Bool is a convenience function that returns
// a boolean value from the Runner.
func (r *Runner) Bool() bool {
	var bgen BoolGen
	r.Draw(&bgen)
	return bool(bgen)
}

// Float64 is a convenience function that returns
// a float64 value from the Runner.
func (r *Runner) Float64() float64 {
	var f Float64Gen
	r.Draw(&f)
	return float64(f)
}

// Float64Gen implements a generator for float64 values.
type Float64Gen float64

var nastyFloats = []float64{
	0.0, 0.5, 1.0 / 3, 10e6, 10e-6, 1.175494351e-38, 2.2250738585072014e-308,
	1.7976931348623157e+308, 3.402823466e+38, 9007199254740992, 1 - 10e-6,
	2 + 10e-6, 1.192092896e-07, 2.2204460492503131e-016,
}

func init() {
	n := []float64{math.NaN(), math.Inf(0)}
	// add 5 NaN and Inf here, since they are more likely to
	// cause failures.
	for i := 0; i < 5; i++ {
		nastyFloats = append(nastyFloats, n...)
	}
	for _, f := range nastyFloats {
		nastyFloats = append(nastyFloats, -f)
	}
}

func (f *Float64Gen) Fill(d Data) {
	fbits := d.Draw(10, func(r *rand.Rand, n int) []byte {
		if n != 10 {
			panic("bad float size")
		}
		flavor := r.Intn(10)
		var f float64
		switch {
		case flavor <= 4:
			f = nastyFloats[r.Intn(len(nastyFloats))]
		case flavor == 5:
			var b [10]byte
			r.Read(b[:])
			return b[:]
		case flavor == 6:
			f = r.Float64() * float64((r.Intn(2)*2)-1)
		case flavor == 7:
			f = r.NormFloat64()
		case flavor == 8:
			u := r.Int63()
			if r.Intn(2) == 1 {
				u = -u
			}
			f = float64(u)
		default:
			u := r.Int63()
			if r.Intn(2) == 1 {
				u = -u
			}
			f = float64(u)
			n := r.NormFloat64()
			n += f
		}
		b := encodefloat64(f)
		return b[:]
	})
	fl, invalid := decodefloat64(fbits)
	if invalid {
		Invalid()
	}
	*f = Float64Gen(fl)
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
	// TODO: handle subnormals so that they're more complex
	sexp := int16((bits >> 52) & 0x7ff)
	var exp uint16
	if sexp == 0 {
		if mant != 0 {
			// subnormal number, use the extra range we get from
			// int16 to signal this
			exp = sint16tolex16(-1024)
		}
	} else if sexp == 0x7ff {
		// infinity and NaN, they're more complex that negative
		// exponent and subnormals
		exp = sint16tolex16(-1025)
	} else {
		// regular exponent
		// unbias
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
			exp = sint16tolex16(sexp)
		}
	}
	binary.BigEndian.PutUint16(b[8:], exp)
	return b
}

func sint16tolex16(s int16) uint16 {
	s *= -1
	exp := uint16(s)
	exp ^= (1 << 15)
	return exp
}

func decodefloat64(b []byte) (float64, bool) {
	fbits := uint64(0)
	sign := b[0]
	if sign != 0 && sign != 1 {
		return 0, true
	}
	fbits = uint64(sign) << 63
	// mantissa only take 7 bytes in our binary packing
	// but binary only lets us read in chunks of 8
	// copy the mantissa value into an empty array
	// and then decode to make sure that we don't
	var mb [8]byte
	copy(mb[:], b[1:8])
	mant := binary.LittleEndian.Uint64(mb[:])

	exp := binary.BigEndian.Uint16(b[8:])
	if exp&(1<<15) != 0 {
		// this is a signed exponent
		// clear the sign bit
		sexp := int16(exp & (^uint16(0) >> 1))
		if sexp == 1024 {
			exp = 0
		} else if sexp == 1025 {
			exp = 0x7ff
		} else {
			// this is a regular negative exponent
			// make into negative number
			sexp *= -1
			// bias
			sexp += 1023
			exp = uint16(sexp)
		}
	} else if exp != 0 {
		// positive exponent
		exp -= 1
		exp += 1023
	} else if mant != 0 {
		mant = 0
	}
	fbits ^= mant & (^uint64(0) >> (64 - 52))
	fbits ^= uint64(exp) << 52
	return math.Float64frombits(fbits), false
}

// Uint64Gen implements a generator for uint64 values.
type Uint64Gen uint64

func (u *Uint64Gen) Fill(d Data) {
	b := d.Draw(8, Uniform)
	*u = Uint64Gen(binary.BigEndian.Uint64(b))
}

// Uint64 is a convenience function that returns
// a uint64 value from the Runner.
func (r *Runner) Uint64() uint64 {
	var u Uint64Gen
	r.Draw(&u)
	return uint64(u)
}

// Int16Gen implements a generator for int16 values.
type Int16Gen int16

func (i *Int16Gen) Fill(d Data) {
	f := d.Draw(2, Uniform)
	*i = Int16Gen(binary.BigEndian.Uint16(f))
}

// Int16 is a convenience function that returns
// a int16 value from the Runner.
func (r *Runner) Int16() int16 {
	var i Int16Gen
	r.Draw(&i)
	return int16(i)
}

// ByteGen implements a generator for byte values.
type ByteGen byte

func (b *ByteGen) Fill(d Data) {
	*b = ByteGen(d.Draw(1, Uniform)[0])
}

// Byte is a convenience function that returns
// a byte value from the Runner.
func (r *Runner) Byte() byte {
	var b ByteGen
	r.Draw(&b)
	return byte(b)
}

// Int63nGen generates a int64 between 0 and N, following
// the pattern of the math/rand function. After Fill
// the value can be read from Value
type Int63nGen struct {
	Value int64
	N     int64
}

func (i *Int63nGen) Fill(d Data) {
	bits := d.Draw(8, func(r *rand.Rand, n int) []byte {
		var b [8]byte
		val := r.Int63n(i.N)
		binary.BigEndian.PutUint64(b[:], uint64(val))
		return b[:]
	})
	val := int64(binary.BigEndian.Uint64(bits))
	if val >= i.N || val < 0 {
		Invalid()
	}
	i.Value = val
}

func biasBool(d Data, f float64) bool {
	bits := d.Draw(1, func(r *rand.Rand, n int) []byte {
		roll := r.Float64()
		b := byte(0)
		if roll < f {
			b = 1
		}
		return []byte{b}
	})
	return bits[0] != 0
}
