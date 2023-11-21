package lzss

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/compress/prefix_code"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"slices"
)

// bite size of c needs to be the greatest common denominator of all ref types and 8
// d consists of bytes
func Decompress(api frontend.API, c []frontend.Variable, cLength frontend.Variable, d []frontend.Variable, dict []byte, huffman HuffmanSettings) (dLength frontend.Variable, err error) {

	dict = augmentDict(dict)

	// assert that c are within range
	for _, cI := range c {
		api.AssertIsBoolean(cI)
	}

	fileCompressionMode := compress.ReadNum(api, c, 8, 1)
	c = c[8:]
	cLength = api.Sub(cLength, 8)
	api.AssertIsBoolean(fileCompressionMode)
	decompressionNotBypassed := api.Sub(1, api.IsZero(fileCompressionMode))

	outTable := logderivlookup.New(api)
	for i := range dict {
		outTable.Insert(dict[i])
	}

	// formatted input
	chars, charsLen := prefix_code.Read(api, c, huffman.chars.lengths)
	lens, lensLen := prefix_code.Read(api, c, huffman.lens.lengths)
	addrs, addrsLen := prefix_code.Read(api, c, huffman.addrs.lengths)
	{ // pad
		width := slices.Max(huffman.chars.lengths)
		compress.PadTables(api, width, lens, lensLen)
		width += slices.Max(huffman.lens.lengths)
		compress.PadTables(api, width, addrs, addrsLen)
	}

	// state variables
	inI := frontend.Variable(0)
	copyLen := frontend.Variable(0) // remaining length of the current copy
	copyLen01 := frontend.Variable(1)
	eof := frontend.Variable(0)
	dLength = 0

	for outI := range d {

		curr := chars.Lookup(inI)[0]

		currIndicatesCp := api.IsZero(evalPlonkExpr(api, curr, curr, -(symbolBackref + symbolDict), 0, 1, symbolBackref*symbolDict)) // (curr-A)(curr-B) = curr^2 - (A+B)curr + AB
		currIndicatesCp = api.Mul(currIndicatesCp, decompressionNotBypassed)
		currIndicatesDr := api.Mul(currIndicatesCp, api.Sub(curr, symbolBackref))
		currIndicatesBr := api.Sub(currIndicatesCp, currIndicatesDr)

		currLen := charsLen.Lookup(inI)[0]
		inILen := api.Add(inI, currLen)
		currIndicatedCpLen := api.Add(1, lens.Lookup(inILen)[0]) // TODO Get rid of the +1

		lenLen := lensLen.Lookup(inILen)[0]
		inIAddr := api.Add(inI, lenLen)
		currIndicatedCpAddr := addrs.Lookup(inIAddr)[0]

		copyLen = api.Select(copyLen01, api.Mul(currIndicatesCp, currIndicatedCpLen), api.Sub(copyLen, 1))
		copyLen01 = api.IsZero(api.MulAcc(api.Neg(copyLen), copyLen, copyLen))

		// copying = copyLen01 ? copyLen==1 : 1			either from previous iterations or starting a new copy
		// copying = copyLen01 ? copyLen : 1
		copying := evalPlonkExpr(api, copyLen01, copyLen, -1, 0, 1, 1)

		copyAddr := api.Mul(api.Sub(outI+len(dict)-1, currIndicatedCpAddr), currIndicatesBr)
		dictCopyAddr := api.Add(currIndicatedCpAddr, api.Sub(currIndicatedCpLen, copyLen))
		copyAddr = api.MulAcc(copyAddr, currIndicatesDr, dictCopyAddr)
		toCopy := outTable.Lookup(copyAddr)[0]

		// write to output
		d[outI] = api.Select(copying, toCopy, curr)
		// WARNING: curr modified by MulAcc
		outTable.Insert(d[outI])

		func() { // EOF Logic

			addrLen := addrsLen.Lookup(inIAddr)[0]
			inINextCp := api.Add(inIAddr, addrLen)
			inINext := api.Select(copying, api.Select(copyLen01, inINextCp, inI), inILen) // inILen is the next char after the currently read byte

			// TODO Try removing this check and requiring the user to pad the input with nonzeros
			// TODO Change inner to mulacc once https://github.com/Consensys/gnark/pull/859 is merged
			if eof == 0 {
				inI = inINext
			} else {
				inI = api.Select(eof, inI, inINext) // if eof, stay put
			}

			eofNow := api.IsZero(api.Sub(inI, cLength))

			dLength = api.Add(dLength, api.Mul(api.Sub(eofNow, eof), outI+1)) // if eof, don't advance dLength
			eof = eofNow
		}()

	}
	return dLength, nil
}

func combineIntoBytes(api frontend.API, c []frontend.Variable, wordNbBits int) []frontend.Variable {
	reader := compress.NewNumReader(api, c, 8, wordNbBits)
	res := make([]frontend.Variable, len(c))
	for i := range res {
		res[i] = reader.Next()
	}
	return res
}

func initAddrTable(api frontend.API, bytes, c []frontend.Variable, wordNbBits int, backrefs []refType) *logderivlookup.Table {
	for i := range backrefs {
		if backrefs[i].nbBitsLength != backrefs[0].nbBitsLength {
			panic("all ref types must have the same length size")
		}
	}
	readers := make([]*compress.NumReader, len(backrefs))
	delimAndLenNbWords := int(8+backrefs[0].nbBitsLength) / wordNbBits
	for i := range backrefs {
		var readerC []frontend.Variable
		if len(c) >= delimAndLenNbWords {
			readerC = c[delimAndLenNbWords:]
		}

		readers[i] = compress.NewNumReader(api, readerC, int(backrefs[i].nbBitsAddress), wordNbBits)
	}

	res := logderivlookup.New(api)

	for i := range c {
		entry := frontend.Variable(0)
		for j := range backrefs {
			isSymb := api.IsZero(api.Sub(bytes[i], backrefs[j].delimiter))
			entry = api.MulAcc(entry, isSymb, readers[j].Next())
		}
		res.Insert(entry)
	}

	return res
}
func evalPlonkExpr(api frontend.API, a, b frontend.Variable, aCoeff, bCoeff, mCoeff, constant int) frontend.Variable {
	if plonkAPI, ok := api.(frontend.PlonkAPI); ok {
		return plonkAPI.EvaluatePlonkExpression(a, b, aCoeff, bCoeff, mCoeff, constant)
	}
	return api.Add(api.Mul(a, aCoeff), api.Mul(b, bCoeff), api.Mul(mCoeff, a, b), constant)
}
