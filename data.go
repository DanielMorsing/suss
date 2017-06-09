package suss

type buffer struct {
	maxLength     int
	drawf         drawFunc
	index         int
	buf           []byte
	intervalStack []int
	intervals     [][2]int
}

type drawFunc func(b *buffer, n int, dist Distribution) []byte

func newBuffer(max int, d drawFunc) *buffer {
	return &buffer{
		maxLength: max,
		drawf:     d,
	}
}

func (b *buffer) Draw(n int, dist Distribution) []byte {
	if n == 0 {
		return nil
	}
	if b.index+n > b.maxLength {
		panic(new(eos))
	}
	byt := b.drawf(b, n, dist)
	b.buf = append(b.buf, byt...)
	return byt
}

func (b *buffer) StartExample() {
	b.intervalStack = append(b.intervalStack, len(b.buf))
}

func (b *buffer) EndExample() {
	stk := b.intervalStack
	top := stk[len(stk)-1]
	b.intervalStack = stk[:len(stk)-1]
	interval := [2]int{top, len(b.buf)}
	b.intervals = append(b.intervals, interval)
}
