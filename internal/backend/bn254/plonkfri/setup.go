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

package plonkfri

import (
	"crypto/sha256"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fri"
	"github.com/consensys/gnark/internal/backend/bn254/cs"
)

type Commitment []fr.Element

type OpeningProof struct {
	Val fr.Element
}

type CommitmentScheme interface {
	Commit(a []fr.Element) Commitment
	Open(c Commitment, p fr.Element) OpeningProof
	Verify(c Commitment, o OpeningProof, point fr.Element) bool
}

type MockCommitment struct{}

func (m MockCommitment) Commit(a []fr.Element) Commitment {
	res := make([]fr.Element, len(a))
	copy(res, a)
	return res
}

func (m MockCommitment) Open(c Commitment, p fr.Element) OpeningProof {
	var r fr.Element
	for i := len(c) - 1; i >= 0; i-- {
		r.Mul(&r, &p).Add(&r, &c[i])
	}
	return OpeningProof{
		Val: r,
	}
}

func (m MockCommitment) Verify(c Commitment, o OpeningProof, point fr.Element) bool {
	var r fr.Element
	for i := len(c) - 1; i >= 0; i-- {
		r.Mul(&r, &point).Add(&r, &c[i])
	}
	return r.Equal(&o.Val)
}

// ProvingKey stores the data needed to generate a proof:
// * the commitment scheme
// * ql, prepended with as many ones as they are public inputs
// * qr, qm, qo prepended with as many zeroes as there are public inputs.
// * qk, prepended with as many zeroes as public inputs, to be completed by the prover
// with the list of public inputs.
// * sigma_1, sigma_2, sigma_3 in both basis
// * the copy constraint permutation
type ProvingKey struct {

	// Verifying Key is embedded into the proving key (needed by Prove)
	Vk *VerifyingKey

	// qr,ql,qm,qo and Qk incomplete (Ls=Lagrange basis big domain, L=Lagrange basis small domain, C=canonical basis)
	EvaluationQlDomainBigBitReversed  []fr.Element
	EvaluationQrDomainBigBitReversed  []fr.Element
	EvaluationQmDomainBigBitReversed  []fr.Element
	EvaluationQoDomainBigBitReversed  []fr.Element
	LQkIncompleteDomainSmall          []fr.Element
	CQl, CQr, CQm, CQo, CQkIncomplete []fr.Element

	// commitment scheme
	Cscheme MockCommitment

	// Domains used for the FFTs
	// 0 -> "small" domain, used for individual polynomials
	// 1 -> "big" domain, used for the computation of the quotient
	Domain [2]fft.Domain

	// s1, s2, s3 (L=Lagrange basis small domain, C=canonical basis, Ls=Lagrange Shifted big domain)
	LId                                                                    []fr.Element
	EvaluationId1BigDomain, EvaluationId2BigDomain, EvaluationId3BigDomain []fr.Element
	EvaluationS1BigDomain, EvaluationS2BigDomain, EvaluationS3BigDomain    []fr.Element

	// position -> permuted position (position in [0,3*sizeSystem-1])
	Permutation []int64
}

// VerifyingKey stores the data needed to verify a proof:
// * The commitment scheme
// * Commitments of ql prepended with as many ones as there are public inputs
// * Commitments of qr, qm, qo, qk prepended with as many zeroes as there are public inputs
// * Commitments to S1, S2, S3
type VerifyingKey struct {

	// Size circuit
	Size              uint64
	SizeInv           fr.Element
	Generator         fr.Element
	NbPublicVariables uint64

	// cosetShift generator of the coset on the small domain
	CosetShift fr.Element

	// commitment scheme
	Cscheme MockCommitment

	// S commitments to S1, S2, S3
	S   [3]Commitment
	Spp [3]fri.ProofOfProximity

	// Id commitments to Id1, Id2, Id3
	Id   [3]Commitment
	Idpp [3]fri.ProofOfProximity

	// Commitments to ql, qr, qm, qo prepended with as many zeroes (ones for l) as there are public inputs.
	// In particular Qk is not complete.
	Ql, Qr, Qm, Qo, QkIncomplete Commitment
	Qpp                          [5]fri.ProofOfProximity // Ql, Qr, Qm, Qo, Qk

	// Iopp scheme (currently one for each size of polynomial)
	Iopp fri.Iopp
}

// Setup sets proving and verifying keys
func Setup(spr *cs.SparseR1CS) (*ProvingKey, *VerifyingKey, error) {

	var pk ProvingKey
	var vk VerifyingKey

	// The verifying key shares data with the proving key
	pk.Vk = &vk

	nbConstraints := len(spr.Constraints)

	// fft domains
	sizeSystem := uint64(nbConstraints + spr.NbPublicVariables) // spr.NbPublicVariables is for the placeholder constraints
	pk.Domain[0] = *fft.NewDomain(sizeSystem)

	// h, the quotient polynomial is of degree 3(n+1)+2, so it's in a 3(n+2) dim vector space,
	// the domain is the next power of 2 superior to 3(n+2). 4*domainNum is enough in all cases
	// except when n<6.
	if sizeSystem < 6 {
		pk.Domain[1] = *fft.NewDomain(8 * sizeSystem)
	} else {
		pk.Domain[1] = *fft.NewDomain(4 * sizeSystem)
	}
	pk.Vk.CosetShift.Set(&pk.Domain[0].FrMultiplicativeGen)

	vk.Size = pk.Domain[0].Cardinality
	vk.SizeInv.SetUint64(vk.Size).Inverse(&vk.SizeInv)
	vk.Generator.Set(&pk.Domain[0].Generator)
	vk.NbPublicVariables = uint64(spr.NbPublicVariables)

	// IOP schemess
	// The +2 is to handle the blinding. I
	vk.Iopp = fri.RADIX_2_FRI.New(pk.Domain[0].Cardinality+2, sha256.New())

	// public polynomials corresponding to constraints: [ placholders | constraints | assertions ]
	pk.EvaluationQlDomainBigBitReversed = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationQrDomainBigBitReversed = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationQmDomainBigBitReversed = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationQoDomainBigBitReversed = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.LQkIncompleteDomainSmall = make([]fr.Element, pk.Domain[0].Cardinality)
	pk.CQkIncomplete = make([]fr.Element, pk.Domain[0].Cardinality)

	for i := 0; i < spr.NbPublicVariables; i++ { // placeholders (-PUB_INPUT_i + qk_i = 0) TODO should return error is size is inconsistant
		pk.EvaluationQlDomainBigBitReversed[i].SetOne().Neg(&pk.EvaluationQlDomainBigBitReversed[i])
		pk.EvaluationQrDomainBigBitReversed[i].SetZero()
		pk.EvaluationQmDomainBigBitReversed[i].SetZero()
		pk.EvaluationQoDomainBigBitReversed[i].SetZero()
		pk.LQkIncompleteDomainSmall[i].SetZero()                 // --> to be completed by the prover
		pk.CQkIncomplete[i].Set(&pk.LQkIncompleteDomainSmall[i]) // --> to be completed by the prover
	}
	offset := spr.NbPublicVariables
	for i := 0; i < nbConstraints; i++ { // constraints

		pk.EvaluationQlDomainBigBitReversed[offset+i].Set(&spr.Coefficients[spr.Constraints[i].L.CoeffID()])
		pk.EvaluationQrDomainBigBitReversed[offset+i].Set(&spr.Coefficients[spr.Constraints[i].R.CoeffID()])
		pk.EvaluationQmDomainBigBitReversed[offset+i].Set(&spr.Coefficients[spr.Constraints[i].M[0].CoeffID()]).
			Mul(&pk.EvaluationQmDomainBigBitReversed[offset+i], &spr.Coefficients[spr.Constraints[i].M[1].CoeffID()])
		pk.EvaluationQoDomainBigBitReversed[offset+i].Set(&spr.Coefficients[spr.Constraints[i].O.CoeffID()])
		pk.LQkIncompleteDomainSmall[offset+i].Set(&spr.Coefficients[spr.Constraints[i].K])
		pk.CQkIncomplete[offset+i].Set(&pk.LQkIncompleteDomainSmall[offset+i])
	}

	pk.Domain[0].FFTInverse(pk.EvaluationQlDomainBigBitReversed[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationQrDomainBigBitReversed[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationQmDomainBigBitReversed[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationQoDomainBigBitReversed[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.CQkIncomplete, fft.DIF)
	fft.BitReverse(pk.EvaluationQlDomainBigBitReversed[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationQrDomainBigBitReversed[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationQmDomainBigBitReversed[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationQoDomainBigBitReversed[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.CQkIncomplete)

	// Commit to the polynomials to set up the verifying key
	pk.CQl = make([]fr.Element, pk.Domain[0].Cardinality)
	pk.CQr = make([]fr.Element, pk.Domain[0].Cardinality)
	pk.CQm = make([]fr.Element, pk.Domain[0].Cardinality)
	pk.CQo = make([]fr.Element, pk.Domain[0].Cardinality)
	copy(pk.CQl, pk.EvaluationQlDomainBigBitReversed)
	copy(pk.CQr, pk.EvaluationQrDomainBigBitReversed)
	copy(pk.CQm, pk.EvaluationQmDomainBigBitReversed)
	copy(pk.CQo, pk.EvaluationQoDomainBigBitReversed)
	vk.Ql = vk.Cscheme.Commit(pk.CQl)
	vk.Qr = vk.Cscheme.Commit(pk.CQr)
	vk.Qm = vk.Cscheme.Commit(pk.CQm)
	vk.Qo = vk.Cscheme.Commit(pk.CQo)
	vk.QkIncomplete = vk.Cscheme.Commit(pk.CQkIncomplete)
	var err error
	vk.Qpp[0], err = vk.Iopp.BuildProofOfProximity(vk.Ql)
	if err != nil {
		return &pk, &vk, err
	}
	vk.Qpp[1], err = vk.Iopp.BuildProofOfProximity(vk.Qr)
	if err != nil {
		return &pk, &vk, err
	}
	vk.Qpp[2], err = vk.Iopp.BuildProofOfProximity(vk.Qm)
	if err != nil {
		return &pk, &vk, err
	}
	vk.Qpp[3], err = vk.Iopp.BuildProofOfProximity(vk.Qo)
	if err != nil {
		return &pk, &vk, err
	}
	vk.Qpp[4], err = vk.Iopp.BuildProofOfProximity(vk.QkIncomplete)
	if err != nil {
		return &pk, &vk, err
	}

	pk.Domain[1].FFT(pk.EvaluationQlDomainBigBitReversed, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationQrDomainBigBitReversed, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationQmDomainBigBitReversed, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationQoDomainBigBitReversed, fft.DIF, true)

	// build permutation. Note: at this stage, the permutation takes in account the placeholders
	buildPermutation(spr, &pk)

	// set s1, s2, s3
	err = computePermutationPolynomials(&pk, &vk)
	if err != nil {
		return &pk, &vk, err
	}

	return &pk, &vk, nil

}

// buildPermutation builds the Permutation associated with a circuit.
//
// The permutation s is composed of cycles of maximum length such that
//
// 			s. (l||r||o) = (l||r||o)
//
//, where l||r||o is the concatenation of the indices of l, r, o in
// ql.l+qr.r+qm.l.r+qo.O+k = 0.
//
// The permutation is encoded as a slice s of size 3*size(l), where the
// i-th entry of l||r||o is sent to the s[i]-th entry, so it acts on a tab
// like this: for i in tab: tab[i] = tab[permutation[i]]
func buildPermutation(spr *cs.SparseR1CS, pk *ProvingKey) {

	nbVariables := spr.NbInternalVariables + spr.NbPublicVariables + spr.NbSecretVariables
	sizeSolution := int(pk.Domain[0].Cardinality)

	// init permutation
	pk.Permutation = make([]int64, 3*sizeSolution)
	for i := 0; i < len(pk.Permutation); i++ {
		pk.Permutation[i] = -1
	}

	// init LRO position -> variable_ID
	lro := make([]int, 3*sizeSolution) // position -> variable_ID
	for i := 0; i < spr.NbPublicVariables; i++ {
		lro[i] = i // IDs of LRO associated to placeholders (only L needs to be taken care of)
	}

	offset := spr.NbPublicVariables
	for i := 0; i < len(spr.Constraints); i++ { // IDs of LRO associated to constraints
		lro[offset+i] = spr.Constraints[i].L.WireID()
		lro[sizeSolution+offset+i] = spr.Constraints[i].R.WireID()
		lro[2*sizeSolution+offset+i] = spr.Constraints[i].O.WireID()
	}

	// init cycle:
	// map ID -> last position the ID was seen
	cycle := make([]int64, nbVariables)
	for i := 0; i < len(cycle); i++ {
		cycle[i] = -1
	}

	for i := 0; i < len(lro); i++ {
		if cycle[lro[i]] != -1 {
			// if != -1, it means we already encountered this value
			// so we need to set the corresponding permutation index.
			pk.Permutation[i] = cycle[lro[i]]
		}
		cycle[lro[i]] = int64(i)
	}

	// complete the Permutation by filling the first IDs encountered
	for i := 0; i < len(pk.Permutation); i++ {
		if pk.Permutation[i] == -1 {
			pk.Permutation[i] = cycle[lro[i]]
		}
	}
}

// computePermutationPolynomials computes the LDE (Lagrange basis) of the permutations
// s1, s2, s3.
//
// 0	1 	..	n-1		|	n	n+1	..	2*n-1		|	2n		2n+1	..		3n-1     |
//  																					 |
//        																				 | Permutation
// s00  s01 ..   s0n-1	   s10 s11 	 ..		s1n-1 		s20 	s21 	..		s2n-1	 v
// \---------------/       \--------------------/        \------------------------/
// 		s1 (LDE)                s2 (LDE)                          s3 (LDE)
func computePermutationPolynomials(pk *ProvingKey, vk *VerifyingKey) error {

	nbElmt := int(pk.Domain[0].Cardinality)

	// sID = [1,..,g^{n-1},s,..,s*g^{n-1},s^2,..,s^2*g^{n-1}]
	pk.LId = getIDSmallDomain(&pk.Domain[0])

	// canonical form of S1, S2, S3
	pk.EvaluationS1BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationS2BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationS3BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	for i := 0; i < nbElmt; i++ {
		pk.EvaluationS1BigDomain[i].Set(&pk.LId[pk.Permutation[i]])
		pk.EvaluationS2BigDomain[i].Set(&pk.LId[pk.Permutation[nbElmt+i]])
		pk.EvaluationS3BigDomain[i].Set(&pk.LId[pk.Permutation[2*nbElmt+i]])
	}

	// Evaluations of Sid1, Sid2, Sid3 on cosets of Domain[1]
	pk.EvaluationId1BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationId2BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	pk.EvaluationId3BigDomain = make([]fr.Element, pk.Domain[1].Cardinality)
	copy(pk.EvaluationId1BigDomain, pk.LId[:nbElmt])
	copy(pk.EvaluationId2BigDomain, pk.LId[nbElmt:2*nbElmt])
	copy(pk.EvaluationId3BigDomain, pk.LId[2*nbElmt:])
	pk.Domain[0].FFTInverse(pk.EvaluationId1BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationId2BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationId3BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	fft.BitReverse(pk.EvaluationId1BigDomain[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationId2BigDomain[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationId3BigDomain[:pk.Domain[0].Cardinality])
	vk.Id[0] = vk.Cscheme.Commit(pk.EvaluationId1BigDomain)
	vk.Id[1] = vk.Cscheme.Commit(pk.EvaluationId2BigDomain)
	vk.Id[2] = vk.Cscheme.Commit(pk.EvaluationId3BigDomain)
	var err error
	vk.Idpp[0], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationId1BigDomain)
	if err != nil {
		return err
	}
	vk.Idpp[1], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationId2BigDomain)
	if err != nil {
		return err
	}
	vk.Idpp[2], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationId3BigDomain)
	if err != nil {
		return err
	}
	pk.Domain[1].FFT(pk.EvaluationId1BigDomain, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationId2BigDomain, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationId3BigDomain, fft.DIF, true)

	pk.Domain[0].FFTInverse(pk.EvaluationS1BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationS2BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	pk.Domain[0].FFTInverse(pk.EvaluationS3BigDomain[:pk.Domain[0].Cardinality], fft.DIF)
	fft.BitReverse(pk.EvaluationS1BigDomain[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationS2BigDomain[:pk.Domain[0].Cardinality])
	fft.BitReverse(pk.EvaluationS3BigDomain[:pk.Domain[0].Cardinality])

	// commit S1, S2, S3
	vk.S[0] = vk.Cscheme.Commit(pk.EvaluationS1BigDomain[:pk.Domain[0].Cardinality])
	vk.S[1] = vk.Cscheme.Commit(pk.EvaluationS2BigDomain[:pk.Domain[0].Cardinality])
	vk.S[2] = vk.Cscheme.Commit(pk.EvaluationS3BigDomain[:pk.Domain[0].Cardinality])
	vk.Spp[0], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationS1BigDomain[:pk.Domain[0].Cardinality])
	if err != nil {
		return err
	}
	vk.Spp[1], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationS2BigDomain[:pk.Domain[0].Cardinality])
	if err != nil {
		return err
	}
	vk.Spp[2], err = vk.Iopp.BuildProofOfProximity(pk.EvaluationS3BigDomain[:pk.Domain[0].Cardinality])
	if err != nil {
		return err
	}
	pk.Domain[1].FFT(pk.EvaluationS1BigDomain, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationS2BigDomain, fft.DIF, true)
	pk.Domain[1].FFT(pk.EvaluationS3BigDomain, fft.DIF, true)

	return nil

}

// getIDSmallDomain returns the Lagrange form of ID on the small domain
func getIDSmallDomain(domain *fft.Domain) []fr.Element {

	res := make([]fr.Element, 3*domain.Cardinality)

	res[0].SetOne()
	res[domain.Cardinality].Set(&domain.FrMultiplicativeGen)
	res[2*domain.Cardinality].Square(&domain.FrMultiplicativeGen)

	for i := uint64(1); i < domain.Cardinality; i++ {
		res[i].Mul(&res[i-1], &domain.Generator)
		res[domain.Cardinality+i].Mul(&res[domain.Cardinality+i-1], &domain.Generator)
		res[2*domain.Cardinality+i].Mul(&res[2*domain.Cardinality+i-1], &domain.Generator)
	}

	return res
}

// NbPublicWitness returns the expected public witness size (number of field elements)
func (vk *VerifyingKey) NbPublicWitness() int {
	return int(vk.NbPublicVariables)
}

// VerifyingKey returns pk.Vk
func (pk *ProvingKey) VerifyingKey() interface{} {
	return pk.Vk
}
