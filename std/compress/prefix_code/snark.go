package prefix_code

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"slices"
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
