package evmprecompiles

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/fields_bn254"
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
	var t frontend.Variable
	ml := make([]*fields_bn254.E6, n/2+1)
	ml[0], t, err = pair.DoubleMLandEasyPart([2]*sw_bn254.G1Affine{P[0], P[1]}, [2]*sw_bn254.G2Affine{Q[0], Q[1]})
	if err != nil {
		panic(err)
	}

	s := t
	res := pair.Ext6.Select(s, pair.Ext6.One(), ml[0])
	for i := 1; i < n/2; i++ {
		ml[i], t, err = pair.DoubleMLandEasyPart([2]*sw_bn254.G1Affine{P[i+1], P[i+2]}, [2]*sw_bn254.G2Affine{Q[i+1], Q[i+2]})
		if err != nil {
			panic(err)
		}
		res = pair.Ext6.Select(t, res, pair.MulTorus(res, ml[i]))
		s = api.And(s, t)
	}
	if n%2 != 0 {
		ml[n/2], t, err = pair.DoubleMLandEasyPart([2]*sw_bn254.G1Affine{P[n/2], P[n/2+1]}, [2]*sw_bn254.G2Affine{Q[n/2], Q[n/2+1]})
		if err != nil {
			panic(err)
		}
		res = pair.Ext6.Select(t, res, pair.MulTorus(res, ml[n/2]))
		s = api.And(s, t)
	}

	value := pair.HardPart(res, s, false)
	one := pair.One()
	pair.AssertIsEqual(value, one)
}
