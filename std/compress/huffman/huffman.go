package huffman

import (
	"github.com/consensys/gnark/std/compress"
)

// copilot code
type huffmanNode struct {
	weight        int // weight is normally the symbol's frequency
	left          *huffmanNode
	right         *huffmanNode
	symbol        int
	nbDescendents int
}

func CreateTree(weights []int) *huffmanNode {
	// Create a list of nodes
	nodes := make(minHeap, len(weights), len(weights)*2)
	for i := 0; i < len(weights); i++ {
		nodes[i] = &huffmanNode{weight: weights[i], symbol: i, nbDescendents: 1}
	}

	nodes.heapify()

	// Create the tree
	for len(nodes) > 1 {

		// remove the first two nodes
		a := nodes[0]
		nodes.popHead()
		b := nodes[0]
		nodes.popHead()

		// Create a new node
		newNode := &huffmanNode{weight: a.weight + b.weight, left: a, right: b,
			nbDescendents: a.nbDescendents + b.nbDescendents}

		// Add the new node
		nodes.push(newNode)
	}

	return nodes[0]
}

type stackElem struct {
	node  *huffmanNode
	depth int
}

func (node *huffmanNode) GetCodeSizes(NbSymbs int) []int {
	// Create the code sizes
	codeSizes := make([]int, NbSymbs)
	stack := make([]stackElem, 0, NbSymbs)
	stack = append(stack, stackElem{node, 0})
	for len(stack) > 0 {
		// pop stack
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e.node.right != nil {
			stack = append(stack, stackElem{e.node.right, e.depth + 1})
		}
		if e.node.left != nil {
			stack = append(stack, stackElem{e.node.left, e.depth + 1})
		}
		if e.node.right == nil && e.node.left == nil {
			codeSizes[e.node.symbol] = e.depth
		}
	}
	return codeSizes
}

func GetCodeLengths(in compress.Stream) []int {
	// create frequency table
	frequencies := make([]int, in.NbSymbs)
	for _, c := range in.D {
		frequencies[c]++
	}

	huffmanTree := CreateTree(frequencies)
	return huffmanTree.GetCodeSizes(in.NbSymbs)
}

// Encode encodes the data using Huffman coding, EXTREMELY INEFFICIENTLY
func Encode(in compress.Stream) compress.Stream {
	// create frequency table
	frequencies := make([]int, in.NbSymbs)
	for _, c := range in.D {
		frequencies[c]++
	}

	huffmanTree := CreateTree(frequencies)
	codes := make([][]int, in.NbSymbs)
	huffmanTree.traverse([]int{}, codes)

	// encode
	out := make([]int, 0)
	for _, c := range in.D {
		out = append(out, codes[c]...)
	}
	return compress.Stream{D: out, NbSymbs: 2}
}

func (node *huffmanNode) traverse(code []int, codes [][]int) {
	if node.left == nil && node.right == nil {
		codes[node.symbol] = make([]int, len(code))
		copy(codes[node.symbol], code)
		return
	}
	if node.left != nil {
		node.left.traverse(append(code, 0), codes)
	}
	if node.right != nil {
		node.right.traverse(append(code, 1), codes)
	}
}

// An minHeap is a min-heap of linear expressions. It facilitates merging k-linear expressions.
//
// The code is identical to https://pkg.go.dev/container/heap but replaces interfaces with concrete
// type to avoid memory overhead.
type minHeap []*huffmanNode

func (h minHeap) less(i, j int) bool {
	return h[i].weight < h[j].weight || (h[i].weight == h[j].weight && h[i].nbDescendents < h[j].nbDescendents) // prevent very deep trees
}
func (h minHeap) swap(i, j int) { h[i], h[j] = h[j], h[i] }

// heapify establishes the heap invariants required by the other routines in this package.
// heapify is idempotent with respect to the heap invariants
// and may be called whenever the heap invariants may have been invalidated.
// The complexity is O(n) where n = len(*h).
func (h *minHeap) heapify() {
	// heapify
	n := len(*h)
	for i := n/2 - 1; i >= 0; i-- {
		h.down(i, n)
	}
}

// push the element x onto the heap.
// The complexity is O(log n) where n = len(*h).
func (h *minHeap) push(x *huffmanNode) {
	*h = append(*h, x)
	h.up(len(*h) - 1)
}

// Pop removes and returns the minimum element (according to Less) from the heap.
// The complexity is O(log n) where n = len(*h).
// Pop is equivalent to Remove(h, 0).
func (h *minHeap) popHead() {
	n := len(*h) - 1
	h.swap(0, n)
	h.down(0, n)
	*h = (*h)[0:n]
}

// fix re-establishes the heap ordering after the element at index i has changed its value.
// Changing the value of the element at index i and then calling fix is equivalent to,
// but less expensive than, calling Remove(h, i) followed by a Push of the new value.
// The complexity is O(log n) where n = len(*h).
func (h *minHeap) fix(i int) {
	if !h.down(i, len(*h)) {
		h.up(i)
	}
}

func (h *minHeap) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.less(j, i) {
			break
		}
		h.swap(i, j)
		j = i
	}
}

func (h *minHeap) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.less(j2, j1) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.less(j, i) {
			break
		}
		h.swap(i, j)
		i = j
	}
	return i > i0
}
