package compress

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
)

func Pack(api frontend.API, words []frontend.Variable, wordLen int) []frontend.Variable {
	wordsPerElem := (api.Compiler().FieldBitLen() - 1) / wordLen
	res := make([]frontend.Variable, 1+(len(words)-1)/wordsPerElem)
	for elemI := range res {
		res[elemI] = 0
		for wordI := 0; wordI < wordsPerElem; wordI++ {
			absWordI := elemI*wordsPerElem + wordI
			if absWordI >= len(words) {
				break
			}
			res[elemI] = api.Add(res[elemI], api.Mul(words[absWordI], 1<<uint(wordLen*wordI)))
		}
	}
	return res
}

func SliceToTable[V any](api frontend.API, slice []V) *logderivlookup.Table {
	table := logderivlookup.New(api)
	for i := range slice {
		table.Insert(slice[i])
	}
	return table
}

type NumReader struct {
	api       frontend.API
	c         []frontend.Variable
	stepCoeff int
	nbWords   int
	nxt       frontend.Variable
}

func NewNumReader(api frontend.API, c []frontend.Variable, numNbBits, wordNbBits int) *NumReader {
	nbWords := numNbBits / wordNbBits
	stepCoeff := 1 << wordNbBits
	nxt := ReadNum(api, c, nbWords, stepCoeff)
	return &NumReader{
		api:       api,
		c:         c,
		stepCoeff: stepCoeff,
		nxt:       nxt,
		nbWords:   nbWords,
	}
}

func ReadNum(api frontend.API, c []frontend.Variable, nbWords, stepCoeff int) frontend.Variable {
	res := frontend.Variable(0)
	coeff := frontend.Variable(1)
	for i := 0; i < nbWords && i < len(c); i++ {
		res = api.MulAcc(res, coeff, c[i])
		coeff = api.Mul(coeff, stepCoeff)
	}
	return res
}

// Next returns the next number in the sequence. returns 0 upon EOF
func (nr *NumReader) Next() frontend.Variable {
	res := nr.nxt
	if len(nr.c) <= nr.nbWords {
		nr.nxt = 0
		return res
	}
	lastSummand := frontend.Variable(0)
	if nr.nbWords > 0 {
		lastSummand = nr.c[nr.nbWords]
	}
	for i := 1; i < nr.nbWords; i++ { // TODO Cache stepCoeff^nbWords
		lastSummand = nr.api.Mul(lastSummand, nr.stepCoeff)
	}

	nr.nxt = nr.api.Add(nr.api.DivUnchecked(nr.api.Sub(res, nr.c[0]), nr.stepCoeff), lastSummand)

	nr.c = nr.c[1:]
	return res
}

func PadTables(api frontend.API, nbZeros int, tables ...*logderivlookup.Table) {
	for _, table := range tables {
		for i := 0; i < nbZeros; i++ {
			table.Insert(0)
		}
	}
}
