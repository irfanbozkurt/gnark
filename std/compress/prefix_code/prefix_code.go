package prefix_code

import (
	"github.com/icza/bitio"
	"golang.org/x/exp/slices"
	"sort"
)

type Node struct {
	Left  *Node
	Right *Node
	Val   uint64
}

const internalNodeVal = ^uint64(0)

// TODO Very inefficient
func NewTreeFromLengths(symbolLengths []int) *Node {
	root := Node{Val: internalNodeVal}

	symbsTable, lengthsTable := LengthsToTables(symbolLengths)
	for code, symb := range symbsTable {
		length := int(lengthsTable[code])
		curr := &root
		for d := 0; d < length; d++ {
			bitAtD := (code >> (length - 1 - d)) & 1

			/*if d == length-1 {
				curr.Val = symb
				break
			}*/

			if bitAtD == 0 {
				if curr.Left == nil {
					curr.Left = &Node{Val: internalNodeVal}
				} else if curr.Left.Val != internalNodeVal {
					panic("invalid tree")
				}
				curr = curr.Left
			} else {
				if curr.Right == nil {
					curr.Right = &Node{Val: internalNodeVal}
				} else if curr.Right.Val != internalNodeVal {
					panic("invalid tree")
				}
				curr = curr.Right
			}
		}
		curr.Val = symb
	}

	return &root
}

func LengthsToTables(symbolLengths []int) (symbsTable, lengthsTable []uint64) {
	codes := LengthsToCodes(symbolLengths)
	maxCodeSize := slices.Max(symbolLengths)
	symbsTable = make([]uint64, 1<<maxCodeSize)
	lengthsTable = make([]uint64, 1<<maxCodeSize)
	for i := range codes {
		l := symbolLengths[i]
		base := codes[i] << uint64(maxCodeSize-l)
		for j := uint64(0); j < 1<<uint64(maxCodeSize-l); j++ {
			symbsTable[base+j] = uint64(i)
			lengthsTable[base+j] = uint64(l)
		}
	}
	return
}

func LengthsToCodes(symbolLengths []int) []uint64 {
	symbs := _range(len(symbolLengths))
	sort.Slice(symbs, func(i, j int) bool {
		return symbolLengths[symbs[i]] < symbolLengths[symbs[j]] || (symbolLengths[symbs[i]] == symbolLengths[symbs[j]] && symbs[i] < symbs[j])
	})
	codes := make([]uint64, len(symbolLengths))
	prevLen := 0
	code := -1
	for _, s := range symbs {
		code++
		length := symbolLengths[s]
		if length >= 64 {
			panic("symbol length too large")
		}
		code <<= uint64(length - prevLen)
		prevLen = length
		codes[s] = uint64(code)
	}
	return codes
}

func _range(i int) []int {
	out := make([]int, i)
	for j := range out {
		out[j] = j
	}
	return out
}

type Table struct {
	Lengths []int // TODO turn into []uint8?
	codes   []uint64
	tree    *Node
}

func (h *Table) Write(w *bitio.Writer, s uint64) {
	w.TryWriteBits(h.codes[s], uint8(h.Lengths[s]))
}

func (h *Table) Read(r *bitio.Reader) uint64 {
	curr := h.tree
	for curr.Left != nil {
		if r.TryReadBool() {
			curr = curr.Right
		} else {
			curr = curr.Left
		}
	}
	return curr.Val
}

func (h *Table) EnsureCodesNotNil() {
	if h.codes == nil {
		h.codes = LengthsToCodes(h.Lengths)
	}
}

func (h *Table) EnsureTreeNotNil() {
	if h.tree == nil {
		h.tree = NewTreeFromLengths(h.Lengths)
	}
}
