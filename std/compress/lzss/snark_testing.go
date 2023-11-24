package lzss

import (
	"compress/gzip"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/profile"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/hash/mimc"
	"os"
	"time"
)

type DecompressionTestCircuit struct {
	C                []frontend.Variable
	D                []byte
	Dict             []byte
	CLength          frontend.Variable
	CheckCorrectness bool
	Pfc              *PrefixCode
}

func (c *DecompressionTestCircuit) Define(api frontend.API) error {
	dBack := make([]frontend.Variable, len(c.D)) // TODO Try smaller constants
	api.Println("maxLen(dBack)", len(dBack))
	dLen, err := Decompress(api, c.C, c.CLength, dBack, c.Dict, c.Pfc)
	if err != nil {
		return err
	}
	if c.CheckCorrectness {
		api.Println("got len", dLen, "expected", len(c.D))
		api.AssertIsEqual(len(c.D), dLen)
		for i := range c.D {
			api.Println("decompressed at", i, "->", dBack[i], "expected", c.D[i], "dBack", dBack[i])
			api.AssertIsEqual(c.D[i], dBack[i])
		}
	}
	return nil
}

func BenchCompressionE2ECompilation(dict []byte, name string, pfc *PrefixCode) (constraint.ConstraintSystem, error) {
	d, err := os.ReadFile(name + "/data.bin")
	if err != nil {
		return nil, err
	}

	// compress

	compressor, err := NewCompressor(dict, BestCompression, pfc)
	if err != nil {
		return nil, err
	}

	c, err := compressor.Compress(d)
	if err != nil {
		return nil, err
	}

	//cStream := ReadIntoStream(c, dict, BestCompression)

	circuit := compressionCircuit{
		C:    make([]frontend.Variable /*cStream.Len()*/, len(c)),
		D:    make([]frontend.Variable, len(d)),
		Dict: make([]byte, len(dict)),
		Pfc:  pfc,
	}

	var start int64
	resetTimer := func() {
		end := time.Now().UnixMilli()
		if start != 0 {
			fmt.Println(end-start, "ms")
		}
		start = end
	}

	// compilation
	fmt.Println("compilation")
	p := profile.Start()
	resetTimer()
	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.WithCapacity(70620000*2))
	if err != nil {
		return nil, err
	}
	p.Stop()
	fmt.Println(1+len(d)/1024, "KB:", p.NbConstraints(), "constraints, estimated", (p.NbConstraints()*600000)/len(d), "constraints for 600KB at", float64(p.NbConstraints())/float64(len(d)), "constraints per uncompressed byte")
	resetTimer()

	outFile, err := os.OpenFile("./testdata/test_cases/"+name+"/e2e_cs.gz", os.O_CREATE, 0600)
	closeFile := func() {
		if err := outFile.Close(); err != nil {
			panic(err)
		}
	}
	defer closeFile()
	if err != nil {
		return nil, err
	}
	gz := gzip.NewWriter(outFile)
	closeZip := func() {
		if err := gz.Close(); err != nil {
			panic(err)
		}
	}
	defer closeZip()
	if _, err = cs.WriteTo(gz); err != nil {
		return nil, err
	}
	return cs, gz.Close()
}

type compressionCircuit struct {
	CChecksum, DChecksum frontend.Variable `gnark:",public"`
	C                    []frontend.Variable
	D                    []frontend.Variable
	Dict                 []byte
	CLen, DLen           frontend.Variable
	Pfc                  *PrefixCode
}

func (c *compressionCircuit) Define(api frontend.API) error {

	fmt.Println("packing")
	cPacked := compress.Pack(api, c.C, 1)
	dPacked := compress.Pack(api, c.D, 8)

	fmt.Println("computing checksum")
	if err := checkSnark(api, cPacked, c.CLen, c.CChecksum); err != nil {
		return err
	}
	if err := checkSnark(api, dPacked, c.DLen, c.DChecksum); err != nil {
		return err
	}

	fmt.Println("decompressing")
	dComputed := make([]frontend.Variable, len(c.D))
	if dComputedLen, err := Decompress(api, c.C, c.CLen, dComputed, c.Dict, c.Pfc); err != nil {
		return err
	} else {
		api.AssertIsEqual(dComputedLen, c.DLen)
		for i := range c.D {
			api.AssertIsEqual(c.D[i], dComputed[i]) // could do this much more efficiently in groth16 using packing :(
		}
	}

	return nil
}

func checkSnark(api frontend.API, e []frontend.Variable, eLen, checksum frontend.Variable) error {
	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	hsh.Write(e...)
	hsh.Write(eLen)
	api.AssertIsEqual(hsh.Sum(), checksum)
	return nil
}
