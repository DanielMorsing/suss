package suss

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type Generator struct {
	rnd     *rand.Rand
	seeder  *rand.Rand
	t       *testing.T
	buf     *buffer
	lastBuf *buffer
}

func NewTest(t *testing.T) *Generator {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	g := &Generator{
		t:      t,
		seeder: r,
	}
	return g
}

func (g *Generator) newData() {
	fmt.Println("newdata")
	g.rnd = rand.New(rand.NewSource(int64(g.seeder.Uint64())))
	g.buf = newBuffer(maxsize, g.regularDraw)
}

const maxsize = 8 << 10

func (g *Generator) Run(f func()) {
	g.newData()
	mutations := 0
	for {
		fmt.Println("run")
		m := g.runOnce(f)
		if m == modeFailed {
			break
		}
		if mutations >= 10 {
			g.newData()
			mutations = 0
			continue
		}
		mutations++
		if g.considerNewBuffer() {
			g.lastBuf = g.buf
		}
		mut := g.newMutator()
		g.buf = newBuffer(maxsize, mut)
	}

	g.t.FailNow()
}

type mode int

const (
	modeOK mode = iota
	modeEOS
	modeFailed
)

func (g *Generator) runOnce(f func()) (m mode) {
	testfail := true
	defer func() {
		r := recover()
		if r == nil {
			if testfail {
				// we tell users to not use t.FailNow
				// but if they do use it
				// give them an error
				panic("use of t.FailNow, t.Fatalf or similar")
			}
			return
		}
		switch r.(type) {
		case *eos:
			m = modeEOS
			return
		case *failed:
			m = modeFailed
			return
		}
		panic(r)
	}()
	f()
	testfail = false
	return modeOK
}

func (g *Generator) Fatalf(format string, i ...interface{}) {
	// TODO: make this hook into the shrinking and gofuzz
	fmt.Printf(format, i...)
	panic(new(failed))
}

func (g *Generator) Draw(n int, dist Distribution) []byte {
	b := g.buf.Draw(n, dist)
	fmt.Println(b)
	return b
}

func (g *Generator) regularDraw(_ *buffer, n int, dist Distribution) []byte {
	b := dist(g.rnd, n)
	return b
}

type Distribution func(r *rand.Rand, n int) []byte

func Uniform(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

func (g *Generator) newMutator() drawFunc {
	mutateDraws := []drawFunc{
		g.drawNew,
		g.drawExisting,
		g.drawLarger,
		g.drawSmaller,
		g.drawZero,
		g.drawConstant,
		g.flipBit,
	}
	return func(b *buffer, n int, dist Distribution) []byte {
		if b.index+n > len(g.lastBuf.buf) {
			return dist(g.rnd, n)
		}
		d := g.seeder.Intn(len(mutateDraws))
		byt := mutateDraws[d](b, n, dist)
		return byt
	}

}

func (g *Generator) drawLarger(b *buffer, n int, dist Distribution) []byte {
	exist := g.lastBuf.buf[b.index : b.index+n]
	r := dist(g.rnd, n)
	if bytes.Compare(r, exist) >= 0 {
		return r
	}
	return g.larger(exist)
}

func (g *Generator) drawSmaller(b *buffer, n int, dist Distribution) []byte {
	exist := g.lastBuf.buf[b.index : b.index+n]
	r := dist(g.rnd, n)
	if bytes.Compare(r, exist) <= 0 {
		return r
	}
	return g.smaller(exist)
}

func (g *Generator) drawNew(b *buffer, n int, dist Distribution) []byte {
	return dist(g.rnd, n)
}

func (g *Generator) drawExisting(b *buffer, n int, dist Distribution) []byte {
	return g.lastBuf.buf[b.index : b.index+n]
}

func (g *Generator) drawZero(b *buffer, n int, dist Distribution) []byte {
	return make([]byte, n)
}

func (g *Generator) drawConstant(b *buffer, n int, dist Distribution) []byte {
	v := byte(g.rnd.Intn(256))
	byt := make([]byte, n)
	for i := 0; i < len(byt); i++ {
		byt[i] = v
	}
	return byt
}

func (g *Generator) flipBit(b *buffer, n int, dist Distribution) []byte {
	byt := make([]byte, n)
	copy(byt, g.lastBuf.buf[b.index:b.index+n])
	i := g.rnd.Intn(n)
	k := g.rnd.Intn(8)
	byt[i] ^= 1 << byte(k)
	return byt
}

func (g *Generator) larger(b []byte) []byte {
	r := make([]byte, len(b))
	drewlarger := false
	for i := 0; i < len(b); i++ {
		if !drewlarger {
			v := 256 - int(b[i])
			r[i] = b[i] + byte(g.rnd.Intn(v))
			if r[i] > b[i] {
				drewlarger = true
			}
		} else {
			r[i] = byte(g.rnd.Intn(256))
		}
	}
	return r
}

func (g *Generator) smaller(b []byte) []byte {
	r := make([]byte, len(b))
	drewsmaller := false
	for i := 0; i < len(b); i++ {
		if !drewsmaller {
			r[i] = byte(g.rnd.Intn(int(b[i]) + 1))
			if r[i] < b[i] {
				drewsmaller = true
			}
		} else {
			r[i] = byte(g.rnd.Intn(256))
		}
	}
	return r

}

func (g *Generator) StartExample() {
	g.buf.StartExample()
}

func (g *Generator) EndExample() {
	g.buf.EndExample()
}

func (g *Generator) considerNewBuffer() bool {
	// TODO: make this actually inspect
	return true
}

type eos struct{}

type failed struct{}
