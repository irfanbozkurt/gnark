package prefix_code

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"golang.org/x/exp/slices"
	"sort"
)

func Read(api frontend.API, c []frontend.Variable, symbolLengths []int) (valuesTable, lengthTable *logderivlookup.Table) {
	width := slices.Max(symbolLengths)
	values := make([]frontend.Variable, len(c))
	length := make([]frontend.Variable, len(c))

	codeSymbs, codeLengths := LengthsToTables(symbolLengths)
	codeSymbsTable := compress.SliceToTable(api, codeSymbs)
	codeLengthsTable := compress.SliceToTable(api, codeLengths)

	curr := frontend.Variable(0)

	for i := 0; i < width && i < len(c); i++ {
		curr = api.Add(curr, api.Mul(c[i], 1<<uint64(width-1-i)))
	}

	for i := 0; i < len(c); i++ {
		values[i] = codeSymbsTable.Lookup(curr)[0]
		length[i] = codeLengthsTable.Lookup(curr)[0]

		// curr[i+1] = 2*curr[i] - bits[i] * 2^width + bits[i+width]
		lsb := frontend.Variable(0)
		if i+width < len(c) {
			lsb = c[i+width]
		}
		curr = api.Add(api.Mul(curr, 2), api.Mul(c[i], -(1<<width)), lsb)
	}

	return compress.SliceToTable(api, values), compress.SliceToTable(api, length)
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
