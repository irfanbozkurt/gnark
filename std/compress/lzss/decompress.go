package lzss

import (
	"bytes"
	"github.com/consensys/gnark/std/compress"
	"github.com/icza/bitio"
	"io"
)

func DecompressGo(data, dict []byte, pfc *PrefixCode) (d []byte, err error) {
	// d[i < 0] = Settings.BackRefSettings.Symbol by convention
	var out bytes.Buffer
	out.Grow(len(data)*6 + len(dict))
	in := bitio.NewReader(bytes.NewReader(data))

	level := Level(in.TryReadByte())
	if level == NoCompression {
		return data[1:], nil
	}

	pfc.ensureTreesNotNil()

	dict = augmentDict(dict)
	backRefType, dictRefType := initRefTypes(len(dict), level)

	dRef := ref{bType: dictRefType}
	bRef := ref{bType: backRefType}

	// read until startAt and write bytes as is

	s, err := pfc.Chars.Read(in)
	for err == nil {
		switch s {
		case symbolBackref:
			// short back ref
			bRef.readFrom(in, pfc)
			for i := 0; i < bRef.length; i++ {
				out.WriteByte(out.Bytes()[out.Len()-bRef.address])
			}
		case symbolDict:
			// dict back ref
			dRef.readFrom(in, pfc)
			out.Write(dict[dRef.address : dRef.address+dRef.length])
		default:
			out.WriteByte(s)
		}
		s, err = pfc.Chars.Read(in)
	}

	if err == io.EOF {
		err = nil
	}

	return out.Bytes(), err
}

func ReadIntoStream(data, dict []byte, pfc *PrefixCode) compress.Stream {
	in := bitio.NewReader(bytes.NewReader(data))

	level := BestCompression
	wordLen := int(BestCompression)

	dict = augmentDict(dict)
	backRefType, dictRefType := initRefTypes(len(dict), level)

	dr := ref{bType: dictRefType}
	br := ref{bType: backRefType}

	levelFromData := Level(in.TryReadByte())
	if levelFromData != NoCompression && levelFromData != level {
		panic("compression mode mismatch")
	}

	out := compress.Stream{
		NbSymbs: 1 << wordLen,
	}

	out.WriteNum(int(levelFromData), 8/wordLen)

	s, err := pfc.Chars.Read(in)

	for err == nil {
		out.WriteNum(int(s), 8/wordLen)

		var r *ref
		switch s {
		case symbolBackref:
			// short back ref
			r = &br
		case symbolDict:
			// dict back ref
			r = &dr
		}
		if r != nil && levelFromData != NoCompression {
			r.readFrom(in, pfc)
			address := r.address
			if r != &dr {
				address--
			}
			out.WriteNum(r.length-1, int(r.bType.nbBitsLength)/wordLen)
			out.WriteNum(address, int(r.bType.nbBitsAddress)/wordLen)
		}

		s, err = pfc.Chars.Read(in)
	}
	if in.TryError != io.EOF {
		panic(in.TryError)
	}
	return out
}
