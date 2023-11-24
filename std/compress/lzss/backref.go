package lzss

import (
	"math"

	"github.com/icza/bitio"
)

const (
	maxInputSize = 1 << 21 // 2Mb
	maxDictSize  = 1 << 22 // 4Mb
)

type refType struct {
	delimiter      byte
	nbBitsAddress  uint8
	nbBitsLength   uint8
	nbBitsBackRef  uint8
	nbBytesBackRef int
	maxAddress     int
	maxLength      int
	dictOnly       bool
}

func newRefType(symbol byte, nbBitsAddress, nbBitsLength uint8, dictOnly bool) refType {
	return refType{
		delimiter:      symbol,
		nbBitsAddress:  nbBitsAddress,
		nbBitsLength:   nbBitsLength,
		nbBitsBackRef:  8 + nbBitsAddress + nbBitsLength,
		nbBytesBackRef: int(8+nbBitsAddress+nbBitsLength+7) / 8,
		maxAddress:     1 << nbBitsAddress,
		maxLength:      1 << nbBitsLength,
		dictOnly:       dictOnly,
	}
}

const (
	symbolDict    = 0xFF
	symbolBackref = 0xFE
)

type ref struct {
	address int
	length  int
	bType   refType
}

func (b *ref) writeTo(w *bitio.Writer, huffman *PrefixCode, i int) {
	huffman.chars.Write(w, uint64(b.bType.delimiter))
	huffman.lens.Write(w, uint64(b.length-1)) // TODO -1 unnecessary with huffman
	address := uint64(b.address)
	if !b.bType.dictOnly {
		address = uint64(i - b.address - 1)
	}
	huffman.addrs.Write(w, address)
}

func (b *ref) readFrom(r *bitio.Reader, huffman *PrefixCode) {

	b.length = int(huffman.lens.Read(r)) + 1
	b.address = int(huffman.addrs.Read(r))
	if !b.bType.dictOnly {
		b.address++
	}
}

// TODO: Pfc-aware savings function
func (b *ref) savings() int {
	if b.length == -1 {
		return math.MinInt // -1 is a special value
	}
	return b.length - b.bType.nbBytesBackRef
}
