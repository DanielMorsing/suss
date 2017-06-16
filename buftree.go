package suss

type bufTree struct {
	nodes []*bufTreeNode
	dead  map[int]bool
}

type bufTreeNode struct {
	edges map[byte]int
	buf   *buffer
}

func newBufTree() *bufTree {
	b := &bufTree{
		dead: make(map[int]bool),
	}
	b.newNode()
	return b
}

func (t *bufTree) add(b *buffer) {
	i := 0
	n := t.nodes[0]
	indices := make([]int, 0, len(b.buf))
	for _, v := range b.buf {
		indices = append(indices, i)
		var ok bool
		i, ok = n.edges[v]
		if !ok {
			i = len(t.nodes)
			t.newNode()
			n.edges[v] = i
		}
		n = t.nodes[i]
		if t.dead[i] {
			break
		}
	}

	if b.status != statusOverrun && !t.dead[i] {
		t.dead[i] = true
		n.buf = b

		for j := len(indices) - 1; j >= 0; j-- {
			idx := indices[j]
			if len(t.nodes[idx].edges) < 256 {
				break
			}
			alldead := true
			for _, v := range t.nodes[idx].edges {
				if !t.dead[v] {
					alldead = false
					break
				}
			}
			if alldead {
				t.dead[idx] = true
				continue
			}
			return
		}
	}
}

func (t *bufTree) newNode() {
	n := &bufTreeNode{
		edges: make(map[byte]int),
	}
	t.nodes = append(t.nodes, n)
}
