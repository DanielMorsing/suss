package suss

type buffer struct {
	status        status
	maxLength     int
	drawf         drawFunc
	index         int
	buf           []byte
	intervalStack []int
	intervals     [][2]int
}

type drawFunc func(b *buffer, n int, smp Sample) []byte

func newBuffer(max int, d drawFunc) *buffer {
	return &buffer{
		maxLength: max,
		drawf:     d,
	}
}

func (b *buffer) Draw(n int, smp Sample) []byte {
	if n == 0 {
		return nil
	}
	if b.index+n > b.maxLength {
		panic(new(eos))
	}
	byt := b.drawf(b, n, smp)
	b.buf = append(b.buf, byt...)
	b.index += n
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

type status int

const (
	statusOverrun status = iota
	statusInvalid
	statusValid
	statusInteresting
)
