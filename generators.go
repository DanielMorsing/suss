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

func (g *Generator) Float64() (f float64) {
	g.StartExample()
	fbits := g.Draw(8, func(r *rand.Rand, n int) []byte {
		if n != 8 {
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
			var b [8]byte
			r.Read(b[:])
			return b[:]
		}
		bits := math.Float64bits(f)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], bits)
		return b[:]
	})
	g.EndExample()
	f = math.Float64frombits(binary.BigEndian.Uint64(fbits))
	return f
}

func (g *Generator) Uint64() uint64 {
	g.StartExample()
	f := g.Draw(8, Uniform)
	g.EndExample()
	return binary.BigEndian.Uint64(f)
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
