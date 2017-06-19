package suss

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

type Runner struct {
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

func NewTest(t *testing.T) *Runner {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := &Runner{
		t:       t,
		seeder:  rnd,
		lastBuf: &buffer{},
		tree:    newBufTree(),
	}
	return r
}

func (r *Runner) newData() {
	r.rnd = rand.New(rand.NewSource(int64(r.seeder.Uint64())))
	r.buf = newBuffer(maxsize, r.regularDraw)
}

const maxsize = 8 << 10

func (r *Runner) Run(f func()) {
	r.startTime = time.Now()
	r.testfunc = f
	r.newData()
	mutations := 0
	for !r.tree.dead[0] {
		r.runOnce()
		r.tree.add(r.buf)
		if r.buf.status == statusInteresting {
			r.lastBuf = r.buf
			break
		}
		if time.Since(r.startTime) > 1*time.Second {
			return
		}
		if mutations >= 10 {
			r.newData()
			mutations = 0
			continue
		}
		mutations++
		if r.considerNewBuffer(r.buf) {
			r.lastBuf = r.buf
		}
		mut := r.newMutator()
		r.buf = newBuffer(maxsize, mut)
	}
	// if we got here, that means that we have an interestinr.buffer
	// That usually means a failing test, now try shrinking it
	if r.buf.status != statusInteresting {
		return
	}
	r.lastBuf.finalize()
	r.shrink()
	r.t.FailNow()
}

func (r *Runner) shrink() {
	defer func() {
		r := recover()
		if _, ok := r.(*complete); ok {
			return
		} else if r != nil {
			panic(r)
		}
	}()
	r.startTime = time.Now()
	change := -1
	for r.change > change {
		change = r.change
		// structured interval delete
		k := len(r.lastBuf.sortedInter) / 2
		for k > 0 {
			i := 0
			for i+k <= len(r.lastBuf.sortedInter) {
				// if I were clever, i'd use some sort of tree
				// for this
				elide := make([]bool, len(r.lastBuf.buf))
				for _, v := range r.lastBuf.sortedInter[i : i+k] {
					for t := v[0]; t < v[1]; t++ {
						elide[t] = true
					}
				}
				byt := make([]byte, 0, len(r.lastBuf.buf))
				for i, v := range r.lastBuf.buf {
					if elide[i] {
						continue
					}
					byt = append(byt, v)
				}
				if !r.tryShrink(byt) {
					i += k
				}
			}
			k /= 2
		}
		r.zeroBlocks()

		// entire buffer minimization
		minimize(r.lastBuf.buf, r.tryShrink, true)

		if change != r.change {
			continue
		}
		// bulk replacing blocks with simpler blocks
		i := 0
		// can't use range here, lastbuf might change
		for i < len(r.lastBuf.blocks) {
			uv := r.lastBuf.blocks[i]
			u, v := uv[0], uv[1]
			buf := r.lastBuf.buf
			block := buf[u:v]
			n := v - u

			byt := make([]byte, len(r.lastBuf.buf))
			for _, v := range r.lastBuf.blocks {
				l := v[1] - v[0]
				origblock := r.lastBuf.buf[v[0]:v[1]]
				if l == n && (bytes.Compare(origblock, block) > 0) {
					byt = append(byt, block...)
				} else {
					byt = append(byt, origblock...)
				}
			}
			r.tryShrink(byt)
			i++
		}
		// replace individual blocks with simpler blocks
		i = 0
		for i < len(r.lastBuf.blocks) {
			uv := r.lastBuf.blocks[i]
			u, v := uv[0], uv[1]
			buf := r.lastBuf.buf
			block := buf[u:v]
			n := v - u
			otherblocks := r.lastBuf.blockStarts[n]
			// find all the blocks simpler than this
			j := sort.Search(len(otherblocks), func(idx int) bool {
				v := otherblocks[idx]
				byt := r.lastBuf.buf[v : v+n]
				return bytes.Compare(byt, block) >= 0
			})
			otherblocks = otherblocks[:j]
			for _, b := range otherblocks {
				byt := append([]byte(nil), r.lastBuf.buf...)
				copy(byt[u:v], r.lastBuf.buf[b:b+n])
				if r.tryShrink(byt) {
					break
				}
			}
			i++
		}
		// shrinking of duplicated blocks
		blockChanged := -1
		for blockChanged != r.change {
			blockChanged = r.change
			blocks := make(map[string][][2]int)
			buf := append([]byte(nil), r.lastBuf.buf...)
			for _, v := range r.lastBuf.blocks {
				s := string(r.lastBuf.buf[v[0]:v[1]])
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
					return r.tryShrink(buf)
				}, false)
			}
		}
		if change != r.change {
			continue
		}
		// shrinking of individual blocks
		i = 0
		for i < len(r.lastBuf.blocks) {
			block := r.lastBuf.blocks[i]
			u, v := block[0], block[1]
			buf := append([]byte(nil), r.lastBuf.buf[u:v]...)
			minimize(buf, func(b []byte) bool {
				byt := append([]byte(nil), r.lastBuf.buf...)
				copy(byt[u:v], b)
				return r.tryShrink(byt)
			}, false)
			i++
		}
	}
}

func (r *Runner) runOnce() {
	testfail := true
	defer func() {
		rec := recover()
		if rec == nil {
			if testfail {
				// we tell users to not use t.FailNow
				// but if they do use it
				// give them an error
				panic("use of t.FailNow, t.Fatalf or similar")
			}
			return
		}
		switch rec.(type) {
		case *eos:
			r.buf.status = statusOverrun
			return
		case *failed:
			r.buf.status = statusInteresting
			return
		case *invalid:
			r.buf.status = statusInvalid
			return
		}
		panic(r)
	}()
	r.testfunc()
	r.buf.status = statusValid
	testfail = false
}

func (r *Runner) tryShrink(byt []byte) bool {
	// TODO slice last_data
	if r.lastBuf.status != statusInteresting {
		panic("whoa")
	}

	i := 0
	noveledge := false
	for _, b := range byt {
		if r.tree.dead[i] {
			return false
		}
		var ok bool
		i, ok = r.tree.nodes[i].edges[b]
		if !ok {
			noveledge = true
			break
		}
	}
	if !noveledge {
		return false
	}

	r.buf = bufFromBytes(byt)
	r.runOnce()
	r.tree.add(r.buf)
	r.buf.finalize()
	if r.considerNewBuffer(r.buf) {
		r.change += 1
		r.lastBuf = r.buf
		return true
	}
	return false
}

func (r *Runner) zeroBlocks() {
	lo := 0
	numBlocks := len(r.lastBuf.blocks)
	hi := numBlocks
	for lo < hi {
		mid := lo + (hi-lo)/2
		byt := append([]byte(nil), r.lastBuf.buf...)
		u := r.lastBuf.blocks[mid][0]
		for i := u; i < len(byt); i++ {
			byt[i] = 0
		}
		if r.tryShrink(byt) {
			// TODO: figure out if this is right
			// if we changed the number of blocks drawn
			// then we could potentially run into out-of-bounds
			// and linear time probing
			if len(r.lastBuf.blocks) != numBlocks {
				break
			}
			hi = mid
		} else {
			lo = mid + 1
		}
	}

	for i := len(r.lastBuf.blocks) - 1; i >= 0; i-- {
		// shrinking might change number of blocks in the
		// last buffer
		if i >= len(r.lastBuf.blocks) {
			i = len(r.lastBuf.blocks)
			continue
		}
		byt := append([]byte(nil), r.lastBuf.buf...)
		block := r.lastBuf.blocks[i]
		u, v := block[0], block[1]
		for i := u; i < v; i++ {
			byt[i] = 0
		}
		r.tryShrink(byt)
	}
}

func (r *Runner) Fatalf(format string, i ...interface{}) {
	// TODO: make this hook into the shrinking and gofuzz
	fmt.Printf(format, i...)
	fmt.Println()
	panic(new(failed))
}

func (r *Runner) Draw(n int, smp Sample) []byte {
	b := r.buf.Draw(n, smp)
	return b
}

func (r *Runner) Invalid() {
	panic(new(invalid))
}

func (r *Runner) regularDraw(b *buffer, n int, smp Sample) []byte {
	res := smp(r.rnd, n)
	return r.rewriteNovelty(b, res)
}

type Sample func(r *rand.Rand, n int) []byte

func Uniform(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

// in case that we happen across a prefix or extension of a buffer
// we generated before, rewrite it with something we haven't seen before
func (r *Runner) rewriteNovelty(b *buffer, result []byte) []byte {
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
	if r.tree.dead[idx] {
		panic("dead node")
	}
	n := r.tree.nodes[idx]
	// walk the tree, looking for places where we
	// would become dead and inserting new values there
	for i, v := range result {
		next, ok := n.edges[v]
		if !ok {
			b.hitNovelty = true
			return result
		}
		nextn := r.tree.nodes[next]
		if r.tree.dead[next] {
			for c := 0; c < 256; c++ {
				if _, ok := n.edges[byte(c)]; !ok {
					result[i] = byte(c)
					b.hitNovelty = true
					return result
				}
				next = n.edges[byte(c)]
				nextn = r.tree.nodes[next]
				if !r.tree.dead[next] {
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

func (r *Runner) newMutator() drawFunc {
	mutateLibrary := []drawFunc{
		r.drawNew,
		r.drawExisting,
		r.drawLarger,
		r.drawSmaller,
		r.drawZero,
		r.drawConstant,
		r.flipBit,
	}
	// choose 3 mutation functions and choose randomly
	// between them on each draw
	// This is the mutation scheme used by conjecture
	perm := r.rnd.Perm(len(mutateLibrary))
	mutateDraws := make([]drawFunc, 3)
	for i := 0; i < 3; i++ {
		mutateDraws[i] = mutateLibrary[perm[i]]
	}

	return func(b *buffer, n int, smp Sample) []byte {
		var res []byte
		if b.index+n > len(r.lastBuf.buf) {
			res = smp(r.rnd, n)
		} else {
			d := r.seeder.Intn(len(mutateDraws))
			res = mutateDraws[d](b, n, smp)
		}
		return r.rewriteNovelty(b, res)
	}

}

func (r *Runner) drawLarger(b *buffer, n int, smp Sample) []byte {
	exist := r.lastBuf.buf[b.index : b.index+n]
	sample := smp(r.rnd, n)
	if bytes.Compare(sample, exist) >= 0 {
		return sample
	}
	return r.larger(exist)
}

func (r *Runner) drawSmaller(b *buffer, n int, smp Sample) []byte {
	exist := r.lastBuf.buf[b.index : b.index+n]
	sample := smp(r.rnd, n)
	if bytes.Compare(sample, exist) <= 0 {
		return sample
	}
	return r.smaller(exist)
}

func (r *Runner) drawNew(b *buffer, n int, smp Sample) []byte {
	return smp(r.rnd, n)
}

func (r *Runner) drawExisting(b *buffer, n int, smp Sample) []byte {
	ret := make([]byte, n)
	copy(ret, r.lastBuf.buf[b.index:b.index+n])
	return ret

}

func (r *Runner) drawZero(b *buffer, n int, smp Sample) []byte {
	return make([]byte, n)
}

func (r *Runner) drawConstant(b *buffer, n int, smp Sample) []byte {
	v := byte(r.rnd.Intn(256))
	byt := make([]byte, n)
	for i := 0; i < len(byt); i++ {
		byt[i] = v
	}
	return byt
}

func (r *Runner) flipBit(b *buffer, n int, smp Sample) []byte {
	byt := make([]byte, n)
	copy(byt, r.lastBuf.buf[b.index:b.index+n])
	i := r.rnd.Intn(n)
	k := r.rnd.Intn(8)
	byt[i] ^= 1 << byte(k)
	return byt
}

func (r *Runner) larger(b []byte) []byte {
	rnd := make([]byte, len(b))
	drewlarger := false
	for i := 0; i < len(b); i++ {
		if !drewlarger {
			v := 256 - int(b[i])
			rnd[i] = b[i] + byte(r.rnd.Intn(v))
			if rnd[i] > b[i] {
				drewlarger = true
			}
		} else {
			rnd[i] = byte(r.rnd.Intn(256))
		}
	}
	return rnd
}

func (r *Runner) smaller(b []byte) []byte {
	rnd := make([]byte, len(b))
	drewsmaller := false
	for i := 0; i < len(b); i++ {
		if !drewsmaller {
			rnd[i] = byte(r.rnd.Intn(int(b[i]) + 1))
			if rnd[i] < b[i] {
				drewsmaller = true
			}
		} else {
			rnd[i] = byte(r.rnd.Intn(256))
		}
	}
	return rnd

}

func (r *Runner) StartExample() {
	r.buf.StartExample()
}

func (r *Runner) EndExample() {
	r.buf.EndExample()
}

func (r *Runner) considerNewBuffer(b *buffer) bool {
	if bytes.Compare(r.lastBuf.buf, b.buf) == 0 {
		return false
	}
	if r.lastBuf.status != b.status {
		return b.status > r.lastBuf.status
	}
	if b.status == statusInvalid {
		return b.index >= r.lastBuf.index
	}
	if b.status == statusOverrun {
		return b.overdraw < r.lastBuf.overdraw
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
