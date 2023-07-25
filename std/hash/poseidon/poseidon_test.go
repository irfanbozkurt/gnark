package poseidon

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/profile"
	"github.com/consensys/gnark/test"
	iden3 "github.com/iden3/go-iden3-crypto/poseidon"
)

type poseidonCircuit struct {
	ExpectedResult frontend.Variable `gnark:"data,public"`
	Data           [10]frontend.Variable
}

func (circuit *poseidonCircuit) Define(api frontend.API) error {
	poseidon := NewPoseidon(api)
	poseidon.Write(circuit.Data[:]...)
	result := poseidon.Sum()
	api.AssertIsEqual(result, circuit.ExpectedResult)
	return nil
}

func TestPoseidon(t *testing.T) {
	assert := test.NewAssert(t)

	var circuit, witness, wrongWitness poseidonCircuit

	var data []*big.Int
	var d big.Int
	modulus := ecc.BN254.ScalarField()
	d.Sub(modulus, big.NewInt(1))
	data = append(data, &d)
	for i := 1; i < 10; i++ {
		d.Add(&d, &d).Mod(&d, modulus)
		data = append(data, &d)
	}

	// running Poseidon (Go)
	expectedh, _ := iden3.Hash(data)

	// assert correctness against correct witness
	for i := 0; i < 10; i++ {
		witness.Data[i] = data[i].String()
	}
	witness.ExpectedResult = expectedh
	assert.SolvingSucceeded(&circuit, &witness, test.WithCurves(ecc.BN254))

	// assert failure against wrong witness
	for i := 0; i < 10; i++ {
		wrongWitness.Data[i] = data[i].Sub(data[i], big.NewInt(1)).String()
	}
	wrongWitness.ExpectedResult = expectedh
	assert.SolvingFailed(&circuit, &wrongWitness, test.WithCurves(ecc.BN254))
}

// bench
func BenchmarkPoseidon(b *testing.B) {
	var c poseidonCircuit
	p := profile.Start()
	_, _ = frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &c)
	p.Stop()
	fmt.Println("Poseidon on BN254 (scs): ", p.NbConstraints())
}
