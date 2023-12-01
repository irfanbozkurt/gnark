package main

import (
	"github.com/consensys/gnark/std/compress/lzss"
	"os"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	dict, err := os.ReadFile("../testdata/dict_naive")
	checkError(err)
	_, err = lzss.BenchCompressionE2ECompilation(dict, "../testdata/test_cases/large")
	checkError(err)
}
