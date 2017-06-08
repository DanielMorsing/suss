package suss

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type Generator struct {
	rnd           *rand.Rand
	seeder        *rand.Rand
	t             *testing.T
	buf           []byte
	lastBuf       []byte
	intervalStack []int
	intervals     [][2]int
	index         int
	draw          func(n int, dist Distribution) []byte
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
	g.rnd = rand.New(rand.NewSource(int64(g.seeder.Uint64())))
	g.intervalStack = nil
	g.intervals = nil
}

func (g *Generator) Run(f func()) {
	g.newData()
	mutations := 0
	for {
		m := g.runOnce(f)
		if m == modeFailed {
			break
		}
		if mutations >= 10 {
			g.newData()
			g.draw = nil
			mutations = 0
			continue
		}
		mutations++
		if g.draw == nil {
			g.lastBuf = g.buf
		}
		g.buf = nil
		g.draw = g.mutateDraw
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
	g.index = 0
	g.intervals = nil
	g.intervalStack = nil
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
	var b []byte
	if g.draw != nil {
		b = g.draw(n, dist)
	} else {
		b = g.regularDraw(n, dist)
	}
	g.buf = append(g.buf, b...)
	return b
}

func (g *Generator) regularDraw(n int, dist Distribution) []byte {
	b := dist(g.rnd, n)
	return b
}

type Distribution func(r *rand.Rand, n int) []byte

func Uniform(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

func (g *Generator) mutateDraw(n int, dist Distribution) []byte {
	d := g.seeder.Intn(len(mutateDraws))
	b := mutateDraws[d](g, n, dist)
	g.index += n
	return b
}

var mutateDraws = []func(g *Generator, n int, dist Distribution) []byte{
	(*Generator).drawNew,
	(*Generator).drawExisting,
	(*Generator).drawZero,
}

func (g *Generator) drawNew(n int, dist Distribution) []byte {
	return dist(g.rnd, n)
}

func (g *Generator) drawExisting(n int, dist Distribution) []byte {
	if g.index+n > len(g.lastBuf) {
		panic(new(eos))
	}
	return g.lastBuf[g.index : g.index+n]
}

func (g *Generator) drawZero(n int, dist Distribution) []byte {
	return make([]byte, n)
}

func (g *Generator) StartExample() {
	g.intervalStack = append(g.intervalStack, len(g.buf))
}

func (g *Generator) EndExample() {
	stk := g.intervalStack
	top := stk[len(stk)-1]
	g.intervalStack = stk[:len(stk)-1]
	interval := [2]int{top, len(g.buf)}
	g.intervals = append(g.intervals, interval)
}

type eos struct{}

type failed struct{}
