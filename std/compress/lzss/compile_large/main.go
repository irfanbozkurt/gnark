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
	_, err = lzss.BenchCompressionE2ECompilation(dict, "../testdata/test_cases/large", getNoPfc())
	checkError(err)
}
func getNoPfc() *lzss.PrefixCode {
	var res lzss.PrefixCode

	res.Chars.Lengths = repeat(8, 256)
	res.Lens.Lengths = repeat(8, 256)
	res.Addrs.Lengths = repeat(19, 1<<19)

	return &res
}

func repeat(x, n int) []int {
	res := make([]int, n)
	for i := range res {
		res[i] = x
	}
	return res
}
