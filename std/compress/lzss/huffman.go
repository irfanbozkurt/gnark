package lzss

import (
	"bytes"
	"compress/gzip"
	"github.com/consensys/gnark/std/compress/huffman"
	"github.com/icza/bitio"
	"io"
	"math/bits"
)

type HuffmanSettings struct {
	charLengths, lenLengths, addrLengths []int
}

func (h *HuffmanSettings) Train(c [][]byte, dictLen int, level Level) {

	charFreq := make([]int, 256)
	lenFreq := make([]int, 256)
	addrFreq := make([]int, 1<<19)

	bShortT, bLongT, bDictT := initBackRefTypes(dictLen, level)
	bShort := backref{bType: bShortT}
	bLong := backref{bType: bLongT}
	bDict := backref{bType: bDictT}

	for _, c := range c {
		in := bitio.NewReader(bytes.NewReader(c))

		s := in.TryReadByte()

		for in.TryError == nil {

			var b *backref
			switch s {
			case symbolShort:
				// short back ref
				b = &bShort
			case symbolLong:
				// long back ref
				b = &bLong
				s = symbolShort
			case symbolDict:
				// dict back ref
				b = &bDict
			}
			charFreq[s]++

			if b != nil {
				b.readFrom(in)
				address := b.address
				if b != &bDict {
					address--
				}
				lenFreq[b.length-1]++
				addrFreq[address]++
			}

			s = in.TryReadByte()
		}
		if in.TryError != io.EOF {
			panic(in.TryError)
		}

	}

	h.charLengths = huffman.CreateTree(charFreq).GetCodeSizes(256)
	h.lenLengths = huffman.CreateTree(lenFreq).GetCodeSizes(256)
	h.addrLengths = huffman.CreateTree(addrFreq).GetCodeSizes(len(addrFreq))
}

func (h *HuffmanSettings) Encode(c, dict []byte) []byte {

	panic("TODO")
}

func intSliceToUint8Slice(in []int) []byte {
	res := make([]uint8, len(in))
	for i, v := range in {
		if v > 255 || v < 0 {
			panic("invalid value")
		}
		res[i] = uint8(v)
	}
	return res
}

func (h *HuffmanSettings) Marshal() []byte {
	var bb bytes.Buffer
	gz := gzip.NewWriter(&bb)
	if logAddrLen := bits.TrailingZeros(uint(len(h.addrLengths))); 1<<logAddrLen != len(h.addrLengths) {
		panic("addr length not power of 2")
	} else if _, err := gz.Write([]byte{byte(logAddrLen)}); err != nil {
		panic(err)
	}

	for _, s := range [][]int{h.charLengths, h.lenLengths, h.addrLengths} {
		for _, i := range s {
			if i > 255 || i < 0 {
				panic("invalid value")
			}
			if _, err := gz.Write([]byte{uint8(i)}); err != nil {
				panic(err)
			}
		}
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	return bb.Bytes()
}

func (h *HuffmanSettings) Unmarshal(d []byte) {

	gz, err := gzip.NewReader(bytes.NewReader(d))
	closeGz := func() {
		if err := gz.Close(); err != nil {
			panic(err)
		}
	}
	if err != nil {
		panic(err)
	}
	defer closeGz()

	var buf [1]byte
	if _, err = gz.Read(buf[:]); err != nil {
		panic(err)
	} else {
		h.addrLengths = make([]int, 1<<buf[0])
	}
	h.charLengths = make([]int, 256)
	h.lenLengths = make([]int, 256)

	for _, s := range [][]int{h.charLengths, h.lenLengths, h.addrLengths} {
		for i := range s {
			if _, err = gz.Read(buf[:]); err != nil {
				panic(err)
			}
			s[i] = int(buf[0])
		}
	}
}
