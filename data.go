package suss

import (
	"bytes"
	"os"
	"sort"
)

type buffer struct {
	status status

	maxLength int
	drawf     drawFunc
	index     int
	buf       []byte
	overdraw  int

	blocks        [][2]int
	blockStarts   map[int][]int
	intervalStack []int
	intervals     map[[2]int]bool
	level         int
	lastlevels    map[int][2]int

	nodeIndex  int
	hitNovelty bool
	finalized  bool
	stdout     string

	sortedInter [][2]int
}

type drawFunc func(b *buffer, n int, smp Sample) []byte

func newBuffer(max int, d drawFunc) *buffer {
	return &buffer{
		maxLength:   max,
		drawf:       d,
		nodeIndex:   -1,
		intervals:   make(map[[2]int]bool),
		lastlevels:  make(map[int][2]int),
		blockStarts: make(map[int][]int),
	}
}

func bufFromBytes(byt []byte) *buffer {
	drawfunc := func(b *buffer, n int, _ Sample) []byte {
		return byt[b.index : b.index+n]
	}
	return newBuffer(len(byt), drawfunc)
}

func (b *buffer) Draw(n int, smp Sample) []byte {
	if n == 0 {
		return nil
	}
	initial := b.index
	if b.index+n > b.maxLength {
		b.overdraw = (b.index + n) - b.maxLength
		panic(new(eos))
	}
	byt := b.drawf(b, n, smp)
	b.blocks = append(b.blocks, [2]int{initial, initial + n})
	b.buf = append(b.buf, byt...)
	b.index += n
	return byt
}

func (b *buffer) StartExample() {
	b.intervalStack = append(b.intervalStack, b.index)
	b.level += 1
}

func (b *buffer) EndExample() {
	stk := b.intervalStack
	top := stk[len(stk)-1]
	b.level -= 1
	b.intervalStack = stk[:len(stk)-1]
	if top == b.index {
		return
	}
	interval := [2]int{top, b.index}
	b.intervals[interval] = true
	lastInter := b.lastlevels[b.level]
	if lastInter[1] == interval[0] {
		mergeinter := [2]int{lastInter[0], interval[1]}
		b.intervals[mergeinter] = true
	}
	b.lastlevels[b.level] = interval
	l := b.index - top
	b.blockStarts[l] = append(b.blockStarts[l], top)
}

func (b *buffer) finalize() {
	if b.finalized {
		return
	}
	b.finalized = true
	sorted := make([][2]int, 0, len(b.intervals))
	for v := range b.intervals {
		sorted = append(sorted, v)
	}
	// sort intervals by length, then earliest position at start
	sort.Slice(sorted, func(i, j int) bool {
		si := sorted[i]
		sj := sorted[j]
		li := si[1] - si[0]
		lj := sj[1] - sj[0]
		if li != lj {
			return lj < li
		}
		return si[0] < sj[0]
	})
	b.sortedInter = sorted
	for l, s := range b.blockStarts {
		sort.Slice(s, func(i, j int) bool {
			u, v := s[i], s[i]+l
			ibuf := b.buf[u:v]
			u, v = s[j], s[j]+l
			jbuf := b.buf[u:v]
			return bytes.Compare(ibuf, jbuf) < 0
		})
	}
}

func (b *buffer) discard() {
	if b.stdout != "" {
		// TODO: figure out erroring here
		// If we can create this file, we should be able to
		// remove it
		os.Remove(b.stdout)
	}
}

type status int

const (
	statusOverrun status = iota
	statusInvalid
	statusValid
	statusInteresting
)
