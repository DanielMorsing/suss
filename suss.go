package suss

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

type Generator struct {
	rnd     *rand.Rand
	seeder  *rand.Rand
	t       *testing.T
	buf     *buffer
	lastBuf *buffer
	tree    *bufTree

	testfunc  func()
	startTime time.Time

	change int
}

func NewTest(t *testing.T) *Generator {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	g := &Generator{
		t:       t,
		seeder:  r,
		lastBuf: &buffer{},
		tree:    newBufTree(),
	}
	return g
}

func (g *Generator) newData() {
	g.rnd = rand.New(rand.NewSource(int64(g.seeder.Uint64())))
	g.buf = newBuffer(maxsize, g.regularDraw)
}

const maxsize = 8 << 10

func (g *Generator) Run(f func()) {
	g.startTime = time.Now()
	g.testfunc = f
	g.newData()
	mutations := 0
	for !g.tree.dead[0] {
		g.runOnce()
		g.tree.add(g.buf)
		if g.buf.status == statusInteresting {
			g.lastBuf = g.buf
			break
		}
		if time.Since(g.startTime) > 1*time.Second {
			return
		}
		if mutations >= 10 {
			g.newData()
			mutations = 0
			continue
		}
		mutations++
		if g.considerNewBuffer(g.buf) {
			g.lastBuf = g.buf
		}
		mut := g.newMutator()
		g.buf = newBuffer(maxsize, mut)
	}
	// if we got here, that means that we have an interesting buffer
	// That usually means a failing test, now try shrinking it
	if g.buf.status != statusInteresting {
		return
	}
	g.lastBuf.finalize()
	g.shrink()
	g.t.FailNow()
}

func (g *Generator) shrink() {
	defer func() {
		r := recover()
		if _, ok := r.(*complete); ok {
			return
		} else if r != nil {
			panic(r)
		}
	}()
	g.startTime = time.Now()
	change := -1
	for g.change > change {
		change = g.change
		// structured interval delete
		k := len(g.lastBuf.sortedInter) / 2
		for k > 0 {
			i := 0
			for i+k <= len(g.lastBuf.sortedInter) {
				// if I were clever, i'd use some sort of tree
				// for this
				elide := make([]bool, len(g.lastBuf.buf))
				for _, v := range g.lastBuf.sortedInter[i : i+k] {
					for t := v[0]; t < v[1]; t++ {
						elide[t] = true
					}
				}
				byt := make([]byte, 0, len(g.lastBuf.buf))
				for i, v := range g.lastBuf.buf {
					if elide[i] {
						continue
					}
					byt = append(byt, v)
				}
				if !g.tryShrink(byt) {
					i += k
				}
			}
			k /= 2
		}
		if change != g.change {
			continue
		}
		// bulk replacing blocks with simpler blocks
		i := 0
		// can't use range here, lastbuf might change
		for i < len(g.lastBuf.blocks) {
			uv := g.lastBuf.blocks[i]
			u, v := uv[0], uv[1]
			buf := g.lastBuf.buf
			block := buf[u:v]
			n := v - u

			byt := make([]byte, len(g.lastBuf.buf))
			for _, v := range g.lastBuf.blocks {
				l := v[1] - v[0]
				origblock := g.lastBuf.buf[v[0]:v[1]]
				if l == n && (bytes.Compare(origblock, block) > 0) {
					byt = append(byt, block...)
				} else {
					byt = append(byt, origblock...)
				}
			}
			g.tryShrink(byt)
			i++
		}
		// replace individual blocks with simpler blocks
		i = 0
		for i < len(g.lastBuf.blocks) {
			uv := g.lastBuf.blocks[i]
			u, v := uv[0], uv[1]
			buf := g.lastBuf.buf
			block := buf[u:v]
			n := v - u
			otherblocks := g.lastBuf.blockStarts[n]
			// find all the blocks simpler than this
			j := sort.Search(len(otherblocks), func(idx int) bool {
				v := otherblocks[idx]
				byt := g.lastBuf.buf[v : v+n]
				return bytes.Compare(byt, block) >= 0
			})
			otherblocks = otherblocks[:j]
			for _, b := range otherblocks {
				byt := append([]byte(nil), g.lastBuf.buf...)
				copy(byt[u:v], g.lastBuf.buf[b:b+n])
				if g.tryShrink(byt) {
					break
				}
			}
			i++
		}
		blockChanged := -1
		for blockChanged != g.change {
			blockChanged = g.change
			blocks := make(map[string][][2]int)
			buf := append([]byte(nil), g.lastBuf.buf...)
			for _, v := range g.lastBuf.blocks {
				s := string(g.lastBuf.buf[v[0]:v[1]])
				blocks[s] = append(blocks[s], v)
			}
			for k, v := range blocks {
				if len(v) == 1 {
					delete(blocks, k)
				}
			}
			for k, s := range blocks {
				minimize([]byte(k), func(b []byte) bool {
					for _, v := range s {
						copy(buf[v[0]:v[1]], b)
					}
					return g.tryShrink(buf)
				}, false)
			}
		}
		if change != g.change {
			continue
		}
		// entire buffer minimization
		minimize(g.lastBuf.buf, g.tryShrink, true)

		// shrinking of individual blocks
		i = 0
		for i < len(g.lastBuf.blocks) {
			block := g.lastBuf.blocks[i]
			u, v := block[0], block[1]
			buf := append([]byte(nil), g.lastBuf.buf[u:v]...)
			minimize(buf, func(b []byte) bool {
				byt := append([]byte(nil), g.lastBuf.buf...)
				copy(byt[u:v], b)
				return g.tryShrink(byt)
			}, false)
			i++
		}
	}
}

func (g *Generator) runOnce() {
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
			g.buf.status = statusOverrun
			return
		case *failed:
			g.buf.status = statusInteresting
			return
		case *invalid:
			g.buf.status = statusInvalid
			return
		}
		panic(r)
	}()
	g.testfunc()
	g.buf.status = statusValid
	testfail = false
}

func (g *Generator) tryShrink(byt []byte) bool {
	// TODO slice last_data
	if g.lastBuf.status != statusInteresting {
		panic("whoa")
	}

	i := 0
	noveledge := false
	for _, b := range byt {
		if g.tree.dead[i] {
			return false
		}
		var ok bool
		i, ok = g.tree.nodes[i].edges[b]
		if !ok {
			noveledge = true
			break
		}
	}
	if !noveledge {
		return false
	}

	g.buf = bufFromBytes(byt)
	g.runOnce()
	g.tree.add(g.buf)
	g.buf.finalize()
	if g.considerNewBuffer(g.buf) {
		g.change += 1
		g.lastBuf = g.buf
		return true
	}
	return false
}

func (g *Generator) Fatalf(format string, i ...interface{}) {
	// TODO: make this hook into the shrinking and gofuzz
	fmt.Printf(format, i...)
	fmt.Println()
	panic(new(failed))
}

func (g *Generator) Draw(n int, smp Sample) []byte {
	b := g.buf.Draw(n, smp)
	return b
}

func (g *Generator) Invalid() {
	panic(new(invalid))
}

func (g *Generator) regularDraw(b *buffer, n int, smp Sample) []byte {
	res := smp(g.rnd, n)
	return g.rewriteNovelty(b, res)
}

type Sample func(r *rand.Rand, n int) []byte

func Uniform(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

// in case that we happen across a prefix or extension of a buffer
// we generated before, rewrite it with something we haven't seen before
func (g *Generator) rewriteNovelty(b *buffer, result []byte) []byte {
	idx := b.nodeIndex
	if idx == -1 {
		if len(b.buf) != 0 {
			fmt.Println(b.buf)
			panic("invalid node index")
		}
		b.nodeIndex = 0
		idx = 0
	}
	// we were novel before, stop the search
	if b.hitNovelty == true {
		return result
	}
	// any opportunity for us to become a dead node
	// goes through previous nodes and we should have
	// rewritten that.
	if g.tree.dead[idx] {
		panic("dead node")
	}
	n := g.tree.nodes[idx]
	// walk the tree, looking for places where we
	// would become dead and inserting new values there
	for i, v := range result {
		next, ok := n.edges[v]
		if !ok {
			b.hitNovelty = true
			return result
		}
		nextn := g.tree.nodes[next]
		if g.tree.dead[next] {
			for c := 0; c < 256; c++ {
				if _, ok := n.edges[byte(c)]; !ok {
					result[i] = byte(c)
					b.hitNovelty = true
					return result
				}
				next = n.edges[byte(c)]
				nextn = g.tree.nodes[next]
				if !g.tree.dead[next] {
					result[i] = byte(c)
					break
				}
			}
		}
		idx = next
		n = nextn
	}
	b.nodeIndex = idx
	return result
}

func (g *Generator) newMutator() drawFunc {
	mutateLibrary := []drawFunc{
		g.drawNew,
		g.drawExisting,
		g.drawLarger,
		g.drawSmaller,
		g.drawZero,
		g.drawConstant,
		g.flipBit,
	}
	// choose 3 mutation functions and choose randomly
	// between them on each draw
	// This is the mutation scheme used by conjecture
	perm := g.rnd.Perm(len(mutateLibrary))
	mutateDraws := make([]drawFunc, 3)
	for i := 0; i < 3; i++ {
		mutateDraws[i] = mutateLibrary[perm[i]]
	}

	return func(b *buffer, n int, smp Sample) []byte {
		var res []byte
		if b.index+n > len(g.lastBuf.buf) {
			res = smp(g.rnd, n)
		} else {
			d := g.seeder.Intn(len(mutateDraws))
			res = mutateDraws[d](b, n, smp)
		}
		return g.rewriteNovelty(b, res)
	}

}

func (g *Generator) drawLarger(b *buffer, n int, smp Sample) []byte {
	exist := g.lastBuf.buf[b.index : b.index+n]
	r := smp(g.rnd, n)
	if bytes.Compare(r, exist) >= 0 {
		return r
	}
	return g.larger(exist)
}

func (g *Generator) drawSmaller(b *buffer, n int, smp Sample) []byte {
	exist := g.lastBuf.buf[b.index : b.index+n]
	r := smp(g.rnd, n)
	if bytes.Compare(r, exist) <= 0 {
		return r
	}
	return g.smaller(exist)
}

func (g *Generator) drawNew(b *buffer, n int, smp Sample) []byte {
	return smp(g.rnd, n)
}

func (g *Generator) drawExisting(b *buffer, n int, smp Sample) []byte {
	ret := make([]byte, n)
	copy(ret, g.lastBuf.buf[b.index:b.index+n])
	return ret

}

func (g *Generator) drawZero(b *buffer, n int, smp Sample) []byte {
	return make([]byte, n)
}

func (g *Generator) drawConstant(b *buffer, n int, smp Sample) []byte {
	v := byte(g.rnd.Intn(256))
	byt := make([]byte, n)
	for i := 0; i < len(byt); i++ {
		byt[i] = v
	}
	return byt
}

func (g *Generator) flipBit(b *buffer, n int, smp Sample) []byte {
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

func (g *Generator) considerNewBuffer(b *buffer) bool {
	if bytes.Compare(g.lastBuf.buf, b.buf) == 0 {
		return false
	}
	if g.lastBuf.status != b.status {
		return b.status > g.lastBuf.status
	}
	if b.status == statusInvalid {
		return b.index >= g.lastBuf.index
	}
	if b.status == statusOverrun {
		return b.overdraw < g.lastBuf.overdraw
	}
	if b.status == statusInteresting {
		// TODO add assertions here
	}
	return true
}

type eos struct{}

type failed struct{}

type invalid struct{}

type complete struct{}
