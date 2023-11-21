package lzss

import (
	"bytes"
	"fmt"
	"math/bits"

	"github.com/consensys/gnark/std/compress/lzss/internal/suffixarray"
	"github.com/icza/bitio"
)

type Compressor struct {
	buf bytes.Buffer
	bw  *bitio.Writer

	inputIndex *suffixarray.Index
	inputSa    [maxInputSize]int32 // suffix array space.

	dictData  []byte
	dictIndex *suffixarray.Index
	dictSa    [maxDictSize]int32 // suffix array space.

	level Level

	huffmanSettings *HuffmanSettings
}

type Level uint8

const (
	NoCompression Level = 0
	// BestCompression allows the compressor to produce a stream of bit-level granularity,
	// giving the compressor this freedom helps it achieve better compression ratios but
	// will impose a high number of constraints on the SNARK decompressor
	BestCompression Level = 1

	GoodCompression        = 2
	GoodSnarkDecompression = 4

	// BestSnarkDecompression forces the compressor to produce byte-aligned output.
	// It is convenient and efficient for the SNARK decompressor but can hurt the compression ratio significantly
	BestSnarkDecompression = 8
)

// NewCompressor returns a new compressor with the given dictionary
func NewCompressor(dict []byte, level Level, huffman *HuffmanSettings) (*Compressor, error) {
	dict = augmentDict(dict)
	if len(dict) > maxDictSize {
		return nil, fmt.Errorf("dict size must be <= %d", maxDictSize)
	}
	c := &Compressor{
		dictData: dict,
	}
	c.buf.Grow(maxInputSize)
	c.dictIndex = suffixarray.New(c.dictData, c.dictSa[:len(c.dictData)])
	c.level = level
	return c, nil
}

func augmentDict(dict []byte) []byte {
	found := uint8(0)
	const mask uint8 = 0b11
	for _, b := range dict {
		if b == symbolDict {
			found |= 0b001
		} else if b == symbolBackref {
			found |= 0b010
		} else {
			continue
		}
		if found == mask {
			return dict
		}
	}

	return append(dict, symbolDict, symbolBackref)
}

func initRefTypes(dictLen int, level Level) (br, dict refType) {
	wordAlign := func(a int) uint8 {
		return (uint8(a) + uint8(level) - 1) / uint8(level) * uint8(level)
	}
	if level == NoCompression {
		wordAlign = func(a int) uint8 {
			return uint8(a)
		}
	}
	br = newRefType(symbolBackref, wordAlign(19), 8, false)
	dict = newRefType(symbolDict, wordAlign(bits.Len(uint(dictLen))), 8, true)
	return
}

// Compress compresses the given data
func (compressor *Compressor) Compress(d []byte) (c []byte, err error) {
	// check input size
	if len(d) > maxInputSize {
		return nil, fmt.Errorf("input size must be <= %d", maxInputSize)
	}

	// reset output buffer
	compressor.buf.Reset()
	compressor.buf.WriteByte(byte(compressor.level))
	if compressor.level == NoCompression {
		compressor.buf.Write(d)
		return compressor.buf.Bytes(), nil
	}
	compressor.bw = bitio.NewWriter(&compressor.buf)

	// build the index
	compressor.inputIndex = suffixarray.New(d, compressor.inputSa[:len(d)])

	backRefType, dictRefType := initRefTypes(len(compressor.dictData), compressor.level)

	dRef := ref{bType: dictRefType, length: -1, address: -1}
	bRef := ref{bType: backRefType, length: -1, address: -1}

	fillBackrefs := func(i int, minLen int) bool {
		dRef.address, dRef.length = compressor.findBackRef(d, i, dictRefType, minLen)
		bRef.address, bRef.length = compressor.findBackRef(d, i, backRefType, minLen)
		return !(dRef.length == -1 && bRef.length == -1)
	}
	bestBackref := func() (ref, int) {
		if dRef.length != -1 && dRef.savings() > bRef.savings() {
			return dRef, dRef.savings()
		}

		return bRef, bRef.savings()
	}

	for i := 0; i < len(d); {
		if !canEncodeSymbol(d[i]) {
			// we must find a ref.
			if !fillBackrefs(i, 1) {
				// we didn't find a ref but can't write the symbol directly
				return nil, fmt.Errorf("could not find a backref at index %d", i)
			}
			best, _ := bestBackref()
			best.writeTo(compressor.bw, compressor.huffmanSettings, i)
			i += best.length
			continue
		}
		if !fillBackrefs(i, -1) {
			// we didn't find a ref, let's write the symbol directly
			compressor.huffmanSettings.chars.write(compressor.bw, uint64(d[i]))
			i++
			continue
		}
		bestAtI, bestSavings := bestBackref()

		if i+1 < len(d) {
			if fillBackrefs(i+1, bestAtI.length+1) {
				if newBest, newSavings := bestBackref(); newSavings > bestSavings {
					// we found an even better ref
					compressor.huffmanSettings.chars.write(compressor.bw, uint64(d[i]))
					i++

					// then emit the ref at i+1
					bestSavings = newSavings
					bestAtI = newBest

					// can we find an even better ref?
					if canEncodeSymbol(d[i]) && i+1 < len(d) {
						if fillBackrefs(i+1, bestAtI.length+1) {
							// we found an even better ref
							if newBest, newSavings := bestBackref(); newSavings > bestSavings {
								compressor.huffmanSettings.chars.write(compressor.bw, uint64(d[i]))
								i++

								// bestSavings = newSavings
								bestAtI = newBest
							}
						}
					}
				}
			} else if i+2 < len(d) && canEncodeSymbol(d[i+1]) {
				// maybe at i+2 ? (we already tried i+1)
				if fillBackrefs(i+2, bestAtI.length+2) {
					if newBest, newSavings := bestBackref(); newSavings > bestSavings {
						// we found a better ref
						// write the symbol at i
						compressor.writeByte(d[i])
						i++
						compressor.writeByte(d[i])
						i++

						// then emit the ref at i+2
						bestAtI = newBest
						// bestSavings = newSavings
					}
				}
			}
		}

		bestAtI.writeTo(compressor.bw, compressor.huffmanSettings, i)
		i += bestAtI.length
	}

	if compressor.bw.TryError != nil {
		return nil, compressor.bw.TryError
	}
	if err := compressor.bw.Close(); err != nil {
		return nil, err
	}

	return compressor.buf.Bytes(), nil
}

// canEncodeSymbol returns true if the symbol can be encoded directly
func canEncodeSymbol(b byte) bool {
	return b != symbolDict && b != symbolBackref
}

func (compressor *Compressor) writeByte(b byte) {
	if !canEncodeSymbol(b) {
		panic("cannot encode symbol")
	}
	compressor.bw.TryWriteByte(b)
}

// findBackRef attempts to find a backref in the window [i-brAddressRange, i+brLengthRange]
// if no backref is found, it returns -1, -1
// else returns the address and length of the backref
func (compressor *Compressor) findBackRef(data []byte, i int, bType refType, minLength int) (addr, length int) {
	if minLength == -1 {
		minLength = bType.nbBytesBackRef
	}

	if i+minLength > len(data) {
		return -1, -1
	}

	windowStart := max(0, i-bType.maxAddress)
	maxRefLen := bType.maxLength

	if i+maxRefLen > len(data) {
		maxRefLen = len(data) - i
	}

	if minLength > maxRefLen {
		return -1, -1
	}

	if bType.dictOnly {
		return compressor.dictIndex.LookupLongest(data[i:i+maxRefLen], minLength, maxRefLen, 0, len(compressor.dictData))
	}

	return compressor.inputIndex.LookupLongest(data[i:i+maxRefLen], minLength, maxRefLen, windowStart, i)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
