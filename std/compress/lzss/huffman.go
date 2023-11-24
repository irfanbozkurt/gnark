package lzss

import (
	"bytes"
	"compress/gzip"
	"github.com/consensys/gnark/std/compress/huffman"
	"github.com/consensys/gnark/std/compress/prefix_code"
	"github.com/icza/bitio"
	"io"
	"math/bits"
)

type PrefixCode struct {
	chars, lens, addrs prefix_code.Table
}

func (h *PrefixCode) TrainHuffman(c [][]byte, dictLen int, level Level) {

	charFreq := make([]int, 256)
	lenFreq := make([]int, 256)
	addrFreq := make([]int, 1<<19)

	bRefT, dRefT := initRefTypes(dictLen, level)
	bShort := ref{bType: bRefT}
	bDict := ref{bType: dRefT}

	for _, c := range c {
		in := bitio.NewReader(bytes.NewReader(c))

		s := in.TryReadByte()

		for in.TryError == nil {

			var b *ref
			switch s {
			case symbolBackref:
				// back ref
				b = &bShort
			case symbolDict:
				// dict ref
				b = &bDict
			}
			charFreq[s]++

			if b != nil {
				b.readFrom(in, h)
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

	h.chars.Lengths = huffman.CreateTree(charFreq).GetCodeSizes(256)
	h.lens.Lengths = huffman.CreateTree(lenFreq).GetCodeSizes(256)
	h.addrs.Lengths = huffman.CreateTree(addrFreq).GetCodeSizes(len(addrFreq))
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

func (h *PrefixCode) Marshal() []byte {
	var bb bytes.Buffer
	gz := gzip.NewWriter(&bb)
	if logAddrLen := bits.TrailingZeros(uint(len(h.addrs.Lengths))); 1<<logAddrLen != len(h.addrs.Lengths) {
		panic("addr length not power of 2")
	} else if _, err := gz.Write([]byte{byte(logAddrLen)}); err != nil {
		panic(err)
	}

	for _, s := range [][]int{h.chars.Lengths, h.lens.Lengths, h.addrs.Lengths} {
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

func (h *PrefixCode) Unmarshal(d []byte) {

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
		h.addrs.Lengths = make([]int, 1<<buf[0])
	}
	h.chars.Lengths = make([]int, 256)
	h.lens.Lengths = make([]int, 256)

	for _, s := range [][]int{h.chars.Lengths, h.lens.Lengths, h.addrs.Lengths} {
		for i := range s {
			if _, err = gz.Read(buf[:]); err != nil {
				panic(err)
			}
			s[i] = int(buf[0])
		}
	}
}

func (h *PrefixCode) ensureCodesNotNil() {
	h.chars.EnsureCodesNotNil()
	h.lens.EnsureCodesNotNil()
	h.addrs.EnsureCodesNotNil()
}

func (h *PrefixCode) ensureTreesNotNil() {
	h.chars.EnsureTreeNotNil()
	h.lens.EnsureTreeNotNil()
	h.addrs.EnsureTreeNotNil()
}
