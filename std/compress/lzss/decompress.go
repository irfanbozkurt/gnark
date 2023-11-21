package lzss

import (
	"bytes"
	"github.com/icza/bitio"
)

func DecompressGo(data, dict []byte, huffman *HuffmanSettings) (d []byte, err error) {
	// d[i < 0] = Settings.BackRefSettings.Symbol by convention
	var out bytes.Buffer
	out.Grow(len(data)*6 + len(dict))
	in := bitio.NewReader(bytes.NewReader(data))

	level := Level(in.TryReadByte())
	if level == NoCompression {
		return data[1:], nil
	}

	dict = augmentDict(dict)
	backRefType, dictRefType := initRefTypes(len(dict), level)

	dRef := ref{bType: dictRefType}
	bRef := ref{bType: backRefType}

	// read until startAt and write bytes as is

	s := in.TryReadByte()
	for in.TryError == nil {
		switch s {
		case symbolBackref:
			// short back ref
			bRef.readFrom(in, huffman)
			for i := 0; i < bRef.length; i++ {
				out.WriteByte(out.Bytes()[out.Len()-bRef.address])
			}
		case symbolDict:
			// dict back ref
			dRef.readFrom(in, huffman)
			out.Write(dict[dRef.address : dRef.address+dRef.length])
		default:
			out.WriteByte(s)
		}
		s = in.TryReadByte()
	}

	return out.Bytes(), nil
}

/*
func ReadIntoStream(data, dict []byte, level Level) compress.Stream {
	in := bitio.NewReader(bytes.NewReader(data))

	wordLen := int(level)

	dict = augmentDict(dict)
	shortBackRefType,  dictBackRefType := initRefTypes(len(dict), level)

	bDict := ref{bType: dictBackRefType}
	bShort := ref{bType: shortBackRefType}
	bLong := ref{bType: longBackRefType}

	levelFromData := Level(in.TryReadByte())
	if levelFromData != NoCompression && levelFromData != level {
		panic("compression mode mismatch")
	}

	out := compress.Stream{
		NbSymbs: 1 << wordLen,
	}

	out.WriteNum(int(levelFromData), 8/wordLen)

	s := in.TryReadByte()

	for in.TryError == nil {
		out.WriteNum(int(s), 8/wordLen)

		var b *ref
		switch s {
		case symbolBackref:
			// short back ref
			b = &bShort
		case symbolLong:
			// long back ref
			b = &bLong
		case symbolDict:
			// dict back ref
			b = &bDict
		}
		if b != nil && levelFromData != NoCompression {
			b.readFrom(in)
			address := b.address
			if b != &bDict {
				address--
			}
			out.WriteNum(b.length-1, int(b.bType.nbBitsLength)/wordLen)
			out.WriteNum(address, int(b.bType.nbBitsAddress)/wordLen)
		}

		s = in.TryReadByte()
	}
	if in.TryError != io.EOF {
		panic(in.TryError)
	}
	return out
}*/
