// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fflonk

import (
	"bytes"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/kzg"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"

	"math/big"
	"math/rand"
	"testing"

	"github.com/consensys/gnark/io"

	"github.com/stretchr/testify/assert"
)

func TestProofSerialization(t *testing.T) {
	// create a  proof
	var proof Proof
	proof.randomize()

	var bb bytes.Buffer
	proof.writeTo(&bb)
	var bproof Proof
	bproof.ReadFrom(&bb)

	assert.NoError(t, io.RoundTripCheck(&proof, func() interface{} { return new(Proof) }))
}

func TestProvingKeySerialization(t *testing.T) {
	// random pk
	var pk ProvingKey
	pk.randomize()

	assert.NoError(t, io.RoundTripCheck(&pk, func() interface{} { return new(ProvingKey) }))
}

func TestVerifyingKeySerialization(t *testing.T) {
	// create a random vk
	var vk VerifyingKey
	vk.randomize()

	assert.NoError(t, io.RoundTripCheck(&vk, func() interface{} { return new(VerifyingKey) }))
}

func (pk *ProvingKey) randomize() {

	var vk VerifyingKey
	vk.randomize()
	pk.Vk = &vk

	pk.Kzg.G1 = make([]curve.G1Affine, 32)
	for i := range pk.Kzg.G1 {
		pk.Kzg.G1[i] = randomG1Point()
	}
	pk.KzgSplitted = make([]kzg.ProvingKey, number_polynomials+3)
}

func (vk *VerifyingKey) randomize() {
	vk.Size = rand.Uint64() //#nosec G404 weak rng is fine here
	vk.SizeInv.SetRandom()
	vk.Generator.SetRandom()
	vk.NbPublicVariables = rand.Uint64()                     //#nosec G404 weak rng is fine here
	vk.CommitmentConstraintIndexes = []uint64{rand.Uint64()} //#nosec G404 weak rng is fine here
	vk.CosetShift.SetRandom()

	vk.Kzg.G1 = randomG1Point()
	vk.Kzg.G2[0] = randomG2Point()
	vk.Kzg.G2[1] = randomG2Point()

	vk.Qpublic = randomG1Point()
}

func (proof *Proof) randomize() {
	proof.LROEntangled = randomG1Point()

	proof.Z = randomG1Point()
	proof.ZEntangled = randomG1Point()
	proof.HEntangled = randomG1Point()
	proof.BsbComEntangled = randomG1Points(rand.Intn(4)) //#nosec G404 weak rng is fine here

	proof.BatchOpeningProof.ClaimedValues = make([][][]fr.Element, 2)
	for i := 0; i < 2; i++ {
		proof.BatchOpeningProof.ClaimedValues[i] = make([][]fr.Element, 3)
		for j := 0; j < 3; j++ {
			proof.BatchOpeningProof.ClaimedValues[i][j] = randomScalars(2)
		}
	}
	proof.BatchOpeningProof.SOpeningProof.ClaimedValues = make([][]fr.Element, 2)
	for i := 0; i < 2; i++ {
		proof.BatchOpeningProof.SOpeningProof.ClaimedValues[i] = randomScalars(2)
	}
	proof.BatchOpeningProof.SOpeningProof.W = randomG1Point()
	proof.BatchOpeningProof.SOpeningProof.WPrime = randomG1Point()
}

func randomG2Point() curve.G2Affine {
	_, _, _, r := curve.Generators()
	r.ScalarMultiplication(&r, big.NewInt(int64(rand.Uint64()))) //#nosec G404 weak rng is fine here
	return r
}

func randomG1Point() curve.G1Affine {
	_, _, r, _ := curve.Generators()
	r.ScalarMultiplication(&r, big.NewInt(int64(rand.Uint64()))) //#nosec G404 weak rng is fine here
	return r
}

func randomG1Points(n int) []curve.G1Affine {
	res := make([]curve.G1Affine, n)
	for i := range res {
		res[i] = randomG1Point()
	}
	return res
}

func randomScalars(n int) []fr.Element {
	v := make([]fr.Element, n)
	one := fr.One()
	for i := 0; i < len(v); i++ {
		if i == 0 {
			v[i].SetRandom()
		} else {
			v[i].Add(&v[i-1], &one)
		}
	}
	return v
}
