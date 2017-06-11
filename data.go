package suss

type buffer struct {
	status        status
	maxLength     int
	drawf         drawFunc
	index         int
	buf           []byte
	intervalStack []int
	intervals     map[[2]int]bool
	level         int
	lastlevels    map[int][2]int
	nodeIndex     int
	hitNovelty    bool
	finalized     bool
}

type drawFunc func(b *buffer, n int, smp Sample) []byte

func newBuffer(max int, d drawFunc) *buffer {
	return &buffer{
		maxLength:  max,
		drawf:      d,
		nodeIndex:  -1,
		intervals:  make(map[[2]int]bool),
		lastlevels: make(map[int][2]int),
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
}

func (b *buffer) finalize() {
	b.finalized = true

}

type status int

const (
	statusOverrun status = iota
	statusInvalid
	statusValid
	statusInteresting
)
