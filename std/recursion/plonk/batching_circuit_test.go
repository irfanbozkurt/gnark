package plonk

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	fr_bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr_bw6761 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
	"github.com/consensys/gnark-crypto/kzg"
	native_plonk "github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/native/sw_bls12377"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/recursion"
	"github.com/consensys/gnark/test"
)

//------------------------------------------------------
// inner circuits

// inner circuit
type InnerCircuit struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
}

func (c *InnerCircuit) Define(api frontend.API) error {
	var res frontend.Variable
	res = c.X
	for i := 0; i < 5; i++ {
		res = api.Mul(res, res)
	}
	api.AssertIsEqual(res, c.Y)

	commitment, err := api.(frontend.Committer).Commit(c.X, res)
	if err != nil {
		return err
	}

	api.AssertIsDifferent(commitment, res)

	return nil
}

// get VK, PK base circuit
func GetInnerCircuitData() (constraint.ConstraintSystem, native_plonk.VerifyingKey, native_plonk.ProvingKey, kzg.SRS) {

	var ic InnerCircuit
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &ic)
	if err != nil {
		panic("compilation failed: " + err.Error())
	}

	srs, err := test.NewKZGSRS(ccs)
	if err != nil {
		panic(err)
	}

	pk, vk, err := native_plonk.Setup(ccs, srs)
	if err != nil {
		panic("setup failed: " + err.Error())
	}

	return ccs, vk, pk, srs
}

// get proofs
func getProofs(ccs constraint.ConstraintSystem, nbInstances int, pk native_plonk.ProvingKey, vk native_plonk.VerifyingKey) ([]native_plonk.Proof, []witness.Witness) {
	proofs := make([]native_plonk.Proof, nbInstances)
	witnesses := make([]witness.Witness, nbInstances)
	for i := 0; i < nbInstances; i++ {
		var assignment InnerCircuit

		var x, y fr_bls12377.Element
		x.SetRandom()
		y.Exp(x, big.NewInt(32))
		assignment.X = x.String()
		assignment.Y = y.String()

		fullWitness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
		if err != nil {
			panic("secret witness failed: " + err.Error())
		}

		publicWitness, err := fullWitness.Public()
		if err != nil {
			panic("public witness failed: " + err.Error())
		}

		proof, err := native_plonk.Prove(ccs, pk, fullWitness)
		if err != nil {
			panic("error proving: " + err.Error())
		}

		proofs[i] = proof
		witnesses[i] = publicWitness

		// sanity check
		err = native_plonk.Verify(proof, vk, publicWitness)
		if err != nil {
			panic("error verifying: " + err.Error())
		}
	}
	return proofs, witnesses
}

//------------------------------------------------------
// outer circuit

// type BatchVerifyCircuit[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT, GtEl algebra.GtElementT] struct {
type BatchVerifyCircuit[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT] struct {

	// dummy proofs, which are selected instead of the real proof, if the
	// corresponding selector is 0. The dummy proofs always pass.
	// There are as many dummy proofs as there are VerifyingKeys.
	// DummyProofs []Proof[FR, G1El, G2El]

	// proofs, verifying keys of the inner circuit
	Proofs        []Proof[FR, G1El, G2El]
	VerifyingKeys VerifyingKey[FR, G1El, G2El]

	// selectors[i]==0/1 means that the i-th circuit is un/instantiated
	// Selectors []frontend.Variable

	// Corresponds to the public inputs of the inner circuit
	PublicInners []emulated.Element[FR]

	// hash of the public inputs of the inner circuits
	HashPub frontend.Variable `gnark:",public"`
}

// func (circuit *BatchVerifyCircuit[FR, G1El, G2El, GtEl]) Define(api frontend.API) error {
func (circuit *BatchVerifyCircuit[FR, G1El, G2El]) Define(api frontend.API) error {

	// get Plonk verifier
	curve, err := algebra.GetCurve[FR, G1El](api)
	if err != nil {
		return err
	}

	// check that hash(PublicInnters)==HashPub
	var fr FR
	h, err := recursion.NewHash(api, fr.Modulus(), true)
	if err != nil {
		return err
	}
	for i := 0; i < len(circuit.PublicInners); i++ {
		toHash := curve.MarshalScalar(circuit.PublicInners[i])
		h.Write(toHash...)
	}
	s := h.Sum()
	api.AssertIsEqual(s, circuit.HashPub)

	return nil
}

// set the outer proof
func TestBatchVerify(t *testing.T) {

	assert := test.NewAssert(t)

	// get ccs, vk, pk, srs
	batchSizeProofs := 1
	ccs, vk, pk, _ := GetInnerCircuitData()

	// get tuples (proof, public_witness)
	proofs, witnesses := getProofs(ccs, batchSizeProofs, pk, vk)

	// hash public inputs of the inner proofs
	h, err := recursion.NewShort(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField())
	assert.NoError(err)
	for i := 0; i < batchSizeProofs; i++ {
		vec := witnesses[i].Vector()
		tvec := vec.(fr_bls12377.Vector)
		for j := 0; j < len(tvec); j++ {
			h.Write(tvec[j].Marshal())
		}
	}
	hashPub := h.Sum(nil)
	var frHashPub fr_bw6761.Element
	frHashPub.SetBytes(hashPub)

	// outer ciruit instantation
	outerCircuit := &BatchVerifyCircuit[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine]{
		PublicInners: make([]emulated.Element[sw_bls12377.ScalarField], batchSizeProofs),
	}
	outerCircuit.Proofs = make([]Proof[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine], batchSizeProofs)
	for i := 0; i < batchSizeProofs; i++ {
		outerCircuit.Proofs[i] = PlaceholderProof[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine](ccs)
	}
	outerCircuit.VerifyingKeys = PlaceholderVerifyingKey[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine](ccs)

	// witness assignment
	var assignmentPubToPrivWitnesses Witness[sw_bls12377.ScalarField]
	for i := 0; i < batchSizeProofs; i++ {
		curWitness, err := ValueOfWitness[sw_bls12377.ScalarField](witnesses[i])
		assert.NoError(err)
		assignmentPubToPrivWitnesses.Public = append(assignmentPubToPrivWitnesses.Public, curWitness.Public...)
	}
	assignmentVerifyingKeys, err := ValueOfVerifyingKey[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine](vk)
	assert.NoError(err)
	assignmentProofs := make([]Proof[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine], batchSizeProofs)
	for i := 0; i < batchSizeProofs; i++ {
		assignmentProofs[i], err = ValueOfProof[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine](proofs[i])
		assert.NoError(err)
	}
	outerAssignment := &BatchVerifyCircuit[sw_bls12377.ScalarField, sw_bls12377.G1Affine, sw_bls12377.G2Affine]{
		Proofs:        assignmentProofs,
		VerifyingKeys: assignmentVerifyingKeys,
		PublicInners:  assignmentPubToPrivWitnesses.Public,
		HashPub:       frHashPub.String(),
	}
	err = test.IsSolved(outerCircuit, outerAssignment, ecc.BW6_761.ScalarField())
	assert.NoError(err)

}
