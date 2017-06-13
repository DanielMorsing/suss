package suss

import (
	"bytes"
	"sort"
)

type minimizer struct {
	length   int
	current  []byte
	cond     func(b []byte) bool
	cautious bool
	changes  int
	seen     map[string]bool
}

func minimize(b []byte, cond func([]byte) bool, cautious bool) []byte {
	m := minimizer{
		length:   len(b),
		current:  b,
		cond:     cond,
		cautious: cautious,
		seen:     make(map[string]bool),
	}
	m.run()
	return m.current
}

func (m *minimizer) run() {
	if len(m.current) == 0 {
		return
	}
	if len(m.current) == 1 {
		// if we are a single byte and it's trivial to try every possiblity
		newb := []byte{0}
		for i := 0; i < int(m.current[0]); i++ {
			newb[0] = byte(i)
			if m.test(newb) {
				break
			}
		}
		return
	}
	// try zeroing
	byt := make([]byte, m.length)
	if m.test(byt) {
		return
	}
	if !m.cautious {
		// try zeroes followed by 1
		byt[m.length-1] = 1
		if m.test(byt) {
			return
		}
	}
	last := len(m.current)
	sig := 0
	for m.current[sig] == 0 {
		sig++
	}
	// note, we use the binary search here for its side
	// effects, we don't care about its return value
	// The function isn't monotonic, so it might not find
	// the longest subsequence
	// Zero out a prefix of the bytes
	current := append([]byte(nil), m.current...)
	sort.Search(last-sig, func(i int) bool {
		b := append([]byte(nil), current...)
		for j := sig; j < i; j++ {
			b[j] = 0
		}
		return !m.test(b)
	})
	// shift right
	current = append([]byte(nil), m.current...)
	sort.Search(len(m.current), func(i int) bool {
		b := make([]byte, len(current))
		copy(b[i:], current)
		return !m.test(b)
	})
	changes := -1
	for !isZero(m.current) && changes < m.changes {
		changes = m.changes

		m.shift()
		m.shrinkIndices(true)

		if !m.cautious {
			m.shrinkIndices(false)
			m.rotateSuffixes()
		}
	}
}

func (m *minimizer) shrinkIndices(timid bool) {
	for i := 0; i < m.length; i++ {
		if m.current[i] == 0 {
			continue
		}

		current := append([]byte(nil), m.current...)
		origSuffix := m.current[i+1:]

		suffix := current[i+1:]
		if !timid {
			for i := range suffix {
				suffix[i] = 255
			}
		}
		suitable := func(c byte) bool {
			current[i] = c
			return m.test(current)
		}
		c := current[i]
		if !suitable(0) && suitable(c-1) {
			// past incorporate may have c
			sort.Search(int(m.current[i]), func(c int) bool {
				if m.current[i] == byte(c) {
					return true
				}
				return suitable(byte(c))
			})
		}
		copy(suffix, origSuffix)
		m.test(current)
	}
}

func (m *minimizer) rotateSuffixes() {
	// find the first significant digit
	sig := 0
	c := byte(0)
	for sig, c = range m.current {
		if c != 0 {
			break
		}
	}
	for i := 1; i < m.length-sig; i++ {
		buf := make([]byte, m.length)
		left := m.current[sig : sig+i]
		right := m.current[sig+i:]
		copy(buf[sig:], right)
		copy(buf[sig+len(right):], left)
		if bytes.Compare(buf, m.current) < 0 {
			m.test(buf)
		}
	}
}

func (m *minimizer) shift() {
	prev := -1
	for prev != m.changes {
		prev = m.changes
		for i := 0; i < m.length; i++ {
			if m.current[i] == 0 {
				continue
			}
			b := append([]byte(nil), m.current...)
			c := b[i]
			for c != 0 {
				c = c >> 1
				b[i] = c
				if m.test(b) {
					break
				}
			}
		}
	}
}

func isZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func (m *minimizer) test(b []byte) bool {
	if len(b) != m.length {
		panic("minimizer changed length")
	}
	if m.seen[string(b)] {
		return false
	}
	cmp := bytes.Compare(b, m.current)
	if cmp > 0 {
		panic("minimizer didn't minimize")
	}
	m.seen[string(b)] = true
	if cmp != 0 && m.cond(b) {
		m.current = append([]byte(nil), b...)
		m.changes += 1
		return true
	}
	return false
}
