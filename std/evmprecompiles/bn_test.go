package evmprecompiles

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/algebra/emulated/sw_emulated"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/test"
)

type ecaddCircuit struct {
	X0       sw_emulated.AffinePoint[emulated.BN254Fp]
	X1       sw_emulated.AffinePoint[emulated.BN254Fp]
	Expected sw_emulated.AffinePoint[emulated.BN254Fp]
}

func (c *ecaddCircuit) Define(api frontend.API) error {
	curve, err := sw_emulated.New[emulated.BN254Fp, emulated.BN254Fr](api, sw_emulated.GetBN254Params())
	if err != nil {
		return err
	}
	res := ECAdd(api, &c.X0, &c.X1)
	curve.AssertIsEqual(res, &c.Expected)
	return nil
}

func testRoutineECAdd() (circ, wit frontend.Circuit) {
	_, _, G, _ := bn254.Generators()
	var u, v fr.Element
	u.SetRandom()
	v.SetRandom()
	var P, Q bn254.G1Affine
	P.ScalarMultiplication(&G, u.BigInt(new(big.Int)))
	Q.ScalarMultiplication(&G, v.BigInt(new(big.Int)))
	var expected bn254.G1Affine
	expected.Add(&P, &Q)
	circuit := ecaddCircuit{}
	witness := ecaddCircuit{
		X0: sw_emulated.AffinePoint[emulated.BN254Fp]{
			X: emulated.ValueOf[emulated.BN254Fp](P.X),
			Y: emulated.ValueOf[emulated.BN254Fp](P.Y),
		},
		X1: sw_emulated.AffinePoint[emulated.BN254Fp]{
			X: emulated.ValueOf[emulated.BN254Fp](Q.X),
			Y: emulated.ValueOf[emulated.BN254Fp](Q.Y),
		},
		Expected: sw_emulated.AffinePoint[emulated.BN254Fp]{
			X: emulated.ValueOf[emulated.BN254Fp](expected.X),
			Y: emulated.ValueOf[emulated.BN254Fp](expected.Y),
		},
	}
	return &circuit, &witness
}

func TestECAddCircuitShort(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, witness := testRoutineECAdd()
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	assert.NoError(err)
}

func TestECAddCircuitFull(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, witness := testRoutineECAdd()
	assert.ProverSucceeded(circuit, witness,
		test.NoFuzzing(), test.NoSerialization(),
		test.WithBackends(backend.GROTH16, backend.PLONK), test.WithCurves(ecc.BN254),
	)
}

type ecmulCircuit struct {
	X0       sw_emulated.AffinePoint[emulated.BN254Fp]
	U        emulated.Element[emulated.BN254Fr]
	Expected sw_emulated.AffinePoint[emulated.BN254Fp]
}

func (c *ecmulCircuit) Define(api frontend.API) error {
	curve, err := sw_emulated.New[emulated.BN254Fp, emulated.BN254Fr](api, sw_emulated.GetBN254Params())
	if err != nil {
		return err
	}
	res := ECMul(api, &c.X0, &c.U)
	curve.AssertIsEqual(res, &c.Expected)
	return nil
}

func testRoutineECMul(t *testing.T) (circ, wit frontend.Circuit) {
	_, _, G, _ := bn254.Generators()
	var u, v fr.Element
	u.SetRandom()
	v.SetRandom()
	var P bn254.G1Affine
	P.ScalarMultiplication(&G, u.BigInt(new(big.Int)))
	var expected bn254.G1Affine
	expected.ScalarMultiplication(&P, v.BigInt(new(big.Int)))
	circuit := ecmulCircuit{}
	witness := ecmulCircuit{
		X0: sw_emulated.AffinePoint[emulated.BN254Fp]{
			X: emulated.ValueOf[emulated.BN254Fp](P.X),
			Y: emulated.ValueOf[emulated.BN254Fp](P.Y),
		},
		U: emulated.ValueOf[emulated.BN254Fr](v),
		Expected: sw_emulated.AffinePoint[emulated.BN254Fp]{
			X: emulated.ValueOf[emulated.BN254Fp](expected.X),
			Y: emulated.ValueOf[emulated.BN254Fp](expected.Y),
		},
	}
	return &circuit, &witness
}

func TestECMulCircuitShort(t *testing.T) {
	assert := test.NewAssert(t)
	circuit, witness := testRoutineECMul(t)
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	assert.NoError(err)
}

func TestECMulCircuitFull(t *testing.T) {
	t.Skip("skipping very long test")
	assert := test.NewAssert(t)
	circuit, witness := testRoutineECMul(t)
	assert.ProverSucceeded(circuit, witness,
		test.NoFuzzing(), test.NoSerialization(),
		test.WithBackends(backend.GROTH16, backend.PLONK), test.WithCurves(ecc.BN254),
	)
}

type ecpairBatch4Circuit struct {
	P [4]sw_bn254.G1Affine
	Q [4]sw_bn254.G2Affine
}

func (c *ecpairBatch4Circuit) Define(api frontend.API) error {
	P := []*sw_bn254.G1Affine{&c.P[0], &c.P[1], &c.P[2], &c.P[3]}
	Q := []*sw_bn254.G2Affine{&c.Q[0], &c.Q[1], &c.Q[2], &c.Q[3]}
	ECPair(api, P, Q, 4)
	return nil
}

func TestECPair4Circuit(t *testing.T) {
	assert := test.NewAssert(t)
	_, _, p, q := bn254.Generators()

	/*
		var u, v fr.Element
		u.SetRandom()
		v.SetRandom()

		p.ScalarMultiplication(&p, u.BigInt(new(big.Int)))
		q.ScalarMultiplication(&q, v.BigInt(new(big.Int)))
	*/

	var np bn254.G1Affine
	np.Neg(&p)

	err := test.IsSolved(&ecpairBatch4Circuit{}, &ecpairBatch4Circuit{
		P: [4]sw_bn254.G1Affine{sw_bn254.NewG1Affine(p), sw_bn254.NewG1Affine(np), sw_bn254.NewG1Affine(p), sw_bn254.NewG1Affine(np)},
		Q: [4]sw_bn254.G2Affine{sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q)},
	}, ecc.BN254.ScalarField())
	assert.NoError(err)
}

type ecpairBatch5Circuit struct {
	P [5]sw_bn254.G1Affine
	Q [5]sw_bn254.G2Affine
}

func (c *ecpairBatch5Circuit) Define(api frontend.API) error {
	P := []*sw_bn254.G1Affine{&c.P[0], &c.P[1], &c.P[2], &c.P[3], &c.P[4]}
	Q := []*sw_bn254.G2Affine{&c.Q[0], &c.Q[1], &c.Q[2], &c.Q[3], &c.Q[4]}
	ECPair(api, P, Q, 5)
	return nil
}

func TestECPair5Circuit(t *testing.T) {
	assert := test.NewAssert(t)
	_, _, p, q := bn254.Generators()

	var u, v fr.Element
	u.SetRandom()
	v.SetRandom()

	p.ScalarMultiplication(&p, u.BigInt(new(big.Int)))
	q.ScalarMultiplication(&q, v.BigInt(new(big.Int)))

	var p2, np bn254.G1Affine
	p2.Double(&p)
	np.Neg(&p)

	err := test.IsSolved(&ecpairBatch5Circuit{}, &ecpairBatch5Circuit{
		P: [5]sw_bn254.G1Affine{sw_bn254.NewG1Affine(p), sw_bn254.NewG1Affine(np), sw_bn254.NewG1Affine(np), sw_bn254.NewG1Affine(np), sw_bn254.NewG1Affine(p2)},
		Q: [5]sw_bn254.G2Affine{sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q), sw_bn254.NewG2Affine(q)},
	}, ecc.BN254.ScalarField())
	assert.NoError(err)
}
