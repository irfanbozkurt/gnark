package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

type refCircuit struct {
	nbConstraints int
	X             frontend.Variable
	Y             frontend.Variable `gnark:",public"`
}

func (circuit *refCircuit) Define(api frontend.API) error {
	for i := 0; i < circuit.nbConstraints; i++ {
		circuit.X = api.Mul(circuit.X, circuit.X)
	}
	api.AssertIsEqual(circuit.X, circuit.Y)
	return nil
}

func referenceCircuit(curve ecc.ID) (constraint.ConstraintSystem, frontend.Circuit, kzg.SRS) {
	const nbConstraints = (1 << 26) - 1000
	circuit := refCircuit{
		nbConstraints: nbConstraints,
	}
	ccs, err := frontend.Compile(curve.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	var good refCircuit
	good.X = 2

	// compute expected Y
	expectedY := new(big.Int).SetUint64(2)
	exp := big.NewInt(1)
	exp.Lsh(exp, nbConstraints)
	expectedY.Exp(expectedY, exp, curve.ScalarField())

	good.Y = expectedY
	srs, err := test.NewKZGSRS(ccs)
	if err != nil {
		panic(err)
	}
	return ccs, &good, srs
}
func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
func main() {
	fmt.Println("starting...")
	ccs, witness, srs := referenceCircuit(ecc.BLS12_377)

	var ccircuit CommCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &ccircuit)
	checkErr(err)
	// cc, r := ccs.(constraint.SparseR1CS).GetConstraints()
	// for _, c := range cc {
	// 	fmt.Printf("%s\n", c.String(r))
	// }
	var witness CommCircuit
	witness.X = 3
	witness.Public = 16
	// parse the assignment and instantiate the witness
	validWitness, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	validPublicWitness, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	srs, err := test.NewKZGSRS(ccs)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	pk, vk, err := plonk.Setup(ccs, srs)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	// prof := profile.Start(profile.CPUProfile, profile.ProfilePath("."))
	proof, err := plonk.Prove(ccs, pk, validWitness)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	// prof.Stop()
	err = plonk.Verify(proof, vk, validPublicWitness)
	checkErr(err)
	fmt.Println("----")
}
