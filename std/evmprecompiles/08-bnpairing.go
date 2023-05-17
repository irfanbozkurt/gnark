package evmprecompiles

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
)

// ECPair implements [ALT_BN128_PAIRING_CHECK] precompile contract at address 0x08.
//
// [ALT_BN128_PAIRING_CHECK]: https://ethereum.github.io/execution-specs/autoapi/ethereum/paris/vm/precompiled_contracts/alt_bn128/index.html#alt-bn128-pairing-check
func ECPair(api frontend.API, P []*sw_bn254.G1Affine, Q []*sw_bn254.G2Affine, n int) {
	pair, err := sw_bn254.NewPairing(api)
	if err != nil {
		panic(err)
	}
	// 1- Check that Pᵢ are on G1 (done in the zkEVM ⚠️ )
	// 2- Check that Qᵢ are on G2
	for i := 0; i < len(Q); i++ {
		pair.AssertIsOnG2(Q[i])
	}

	// 3- Check that ∏ᵢ e(Pᵢ, Qᵢ) == 1
	ml, selector, err := pair.DoubleMLandEasyPart([2]*sw_bn254.G1Affine{P[0], P[1]}, [2]*sw_bn254.G2Affine{Q[0], Q[1]})
	if err != nil {
		panic(err)
	}
	res := pair.HardPart(ml, selector, false)

	for i := 1; i < n/2; i += 2 {
		ml, selector, err = pair.DoubleMLandEasyPart([2]*sw_bn254.G1Affine{P[i+1], P[i+2]}, [2]*sw_bn254.G2Affine{Q[i+1], Q[i+2]})
		if err != nil {
			panic(err)
		}
		tmp := res
		res = pair.HardPart(ml, selector, false)
		res = pair.Mul(tmp, res)
	}

	if n%2 != 0 {
		ml, selector, err = pair.SingleMLandEasyPart(P[n/2+2], Q[n/2+2])
		if err != nil {
			panic(err)
		}
		tmp := res
		res = pair.HardPart(ml, selector, false)
		res = pair.Mul(tmp, res)
	}

	one := pair.One()
	pair.AssertIsEqual(res, one)
}
