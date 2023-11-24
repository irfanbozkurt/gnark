package prefix_code

import (
	"bytes"
	"github.com/icza/bitio"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNoCodingLength2(t *testing.T) {
	table := Table{Lengths: []int{2, 2, 2, 2}}
	table.EnsureCodesNotNil() // for writing
	table.EnsureTreeNotNil()  // for reading

	var bb bytes.Buffer
	w := bitio.NewWriter(&bb)
	table.Write(w, 3)
	assert.NoError(t, w.Close())

	r := bitio.NewReader(bytes.NewReader(bb.Bytes()))
	assert.Equal(t, uint64(3), table.Read(r))
}

func TestLengthsToTables(t *testing.T) {
	const bitLen = 16
	const size = 1 << bitLen
	lengths := make([]int, size)
	for i := range lengths {
		lengths[i] = bitLen
	}

	symbs, lengthsByCode := LengthsToTables(lengths)
	_, _ = symbs, lengthsByCode
}
