// Copyright 2020 Consensys Software Inc.
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

// Code generated by gnark DO NOT EDIT

package mpcsetup

import (
	"crypto/sha256"
	"errors"
	curve "github.com/consensys/gnark-crypto/ecc/bw6-633"
	"github.com/consensys/gnark-crypto/ecc/bw6-633/fr"
	"math"
	"math/big"
)

// Phase1 represents the Phase1 of the MPC described in
// https://eprint.iacr.org/2017/1050.pdf
//
// Also known as "Powers of Tau"
type Phase1 struct {
	Parameters struct {
		G1 struct {
			Tau      []curve.G1Affine // {[τ⁰]₁, [τ¹]₁, [τ²]₁, …, [τ²ⁿ⁻²]₁}
			AlphaTau []curve.G1Affine // {α[τ⁰]₁, α[τ¹]₁, α[τ²]₁, …, α[τⁿ⁻¹]₁}
			BetaTau  []curve.G1Affine // {β[τ⁰]₁, β[τ¹]₁, β[τ²]₁, …, β[τⁿ⁻¹]₁}
		}
		G2 struct {
			Tau  []curve.G2Affine // {[τ⁰]₂, [τ¹]₂, [τ²]₂, …, [τⁿ⁻¹]₂}
			Beta curve.G2Affine   // [β]₂
		}
	}
	PublicKeys struct {
		Tau, Alpha, Beta PublicKey
	}
	Hash []byte // sha256 hash
}

// InitPhase1 initialize phase 1 of the MPC. This is called once by the coordinator before
// any randomness contribution is made (see Contribute()).
func InitPhase1(power int) (phase1 Phase1) {
	N := int(math.Pow(2, float64(power)))

	// Generate key pairs
	var tau, alpha, beta fr.Element
	tau.SetOne()
	alpha.SetOne()
	beta.SetOne()
	phase1.PublicKeys.Tau = newPublicKey(tau, nil, 1)
	phase1.PublicKeys.Alpha = newPublicKey(alpha, nil, 2)
	phase1.PublicKeys.Beta = newPublicKey(beta, nil, 3)

	// First contribution use generators
	_, _, g1, g2 := curve.Generators()
	phase1.Parameters.G2.Beta.Set(&g2)
	phase1.Parameters.G1.Tau = make([]curve.G1Affine, 2*N-1)
	phase1.Parameters.G2.Tau = make([]curve.G2Affine, N)
	phase1.Parameters.G1.AlphaTau = make([]curve.G1Affine, N)
	phase1.Parameters.G1.BetaTau = make([]curve.G1Affine, N)
	for i := 0; i < len(phase1.Parameters.G1.Tau); i++ {
		phase1.Parameters.G1.Tau[i].Set(&g1)
	}
	for i := 0; i < len(phase1.Parameters.G2.Tau); i++ {
		phase1.Parameters.G2.Tau[i].Set(&g2)
		phase1.Parameters.G1.AlphaTau[i].Set(&g1)
		phase1.Parameters.G1.BetaTau[i].Set(&g1)
	}

	phase1.Parameters.G2.Beta.Set(&g2)

	// Compute hash of Contribution
	phase1.Hash = phase1.hash()

	return
}

// Contribute contributes randomness to the phase1 object. This mutates phase1.
func (phase1 *Phase1) Contribute() {
	N := len(phase1.Parameters.G2.Tau)

	// Generate key pairs
	var tau, alpha, beta fr.Element
	tau.SetRandom()
	alpha.SetRandom()
	beta.SetRandom()
	phase1.PublicKeys.Tau = newPublicKey(tau, phase1.Hash[:], 1)
	phase1.PublicKeys.Alpha = newPublicKey(alpha, phase1.Hash[:], 2)
	phase1.PublicKeys.Beta = newPublicKey(beta, phase1.Hash[:], 3)

	// Compute powers of τ, ατ, and βτ
	taus := powers(tau, 2*N-1)
	alphaTau := make([]fr.Element, N)
	betaTau := make([]fr.Element, N)
	for i := 0; i < N; i++ {
		alphaTau[i].Mul(&taus[i], &alpha)
		betaTau[i].Mul(&taus[i], &beta)
	}

	// Update using previous parameters
	// TODO @gbotrel working with jacobian points here will help with perf.
	scaleG1InPlace(phase1.Parameters.G1.Tau, taus)
	scaleG2InPlace(phase1.Parameters.G2.Tau, taus[0:N])
	scaleG1InPlace(phase1.Parameters.G1.AlphaTau, alphaTau)
	scaleG1InPlace(phase1.Parameters.G1.BetaTau, betaTau)
	var betaBI big.Int
	beta.BigInt(&betaBI)
	phase1.Parameters.G2.Beta.ScalarMultiplication(&phase1.Parameters.G2.Beta, &betaBI)

	// Compute hash of Contribution
	phase1.Hash = phase1.hash()
}

func VerifyPhase1(c0, c1 *Phase1, c ...*Phase1) error {
	contribs := append([]*Phase1{c0, c1}, c...)
	for i := 0; i < len(contribs)-1; i++ {
		if err := verifyPhase1(contribs[i], contribs[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// verifyPhase1 checks that a contribution is based on a known previous Phase1 state.
func verifyPhase1(current, contribution *Phase1) error {
	// Compute R for τ, α, β
	tauR := genR(contribution.PublicKeys.Tau.SG, contribution.PublicKeys.Tau.SXG, current.Hash[:], 1)
	alphaR := genR(contribution.PublicKeys.Alpha.SG, contribution.PublicKeys.Alpha.SXG, current.Hash[:], 2)
	betaR := genR(contribution.PublicKeys.Beta.SG, contribution.PublicKeys.Beta.SXG, current.Hash[:], 3)

	// Check for knowledge of toxic parameters
	if !sameRatio(contribution.PublicKeys.Tau.SG, contribution.PublicKeys.Tau.SXG, contribution.PublicKeys.Tau.XR, tauR) {
		return errors.New("couldn't verify public key of τ")
	}
	if !sameRatio(contribution.PublicKeys.Alpha.SG, contribution.PublicKeys.Alpha.SXG, contribution.PublicKeys.Alpha.XR, alphaR) {
		return errors.New("couldn't verify public key of α")
	}
	if !sameRatio(contribution.PublicKeys.Beta.SG, contribution.PublicKeys.Beta.SXG, contribution.PublicKeys.Beta.XR, betaR) {
		return errors.New("couldn't verify public key of β")
	}

	// Check for valid updates using previous parameters
	if !sameRatio(contribution.Parameters.G1.Tau[1], current.Parameters.G1.Tau[1], tauR, contribution.PublicKeys.Tau.XR) {
		return errors.New("couldn't verify that [τ]₁ is based on previous contribution")
	}
	if !sameRatio(contribution.Parameters.G1.AlphaTau[0], current.Parameters.G1.AlphaTau[0], alphaR, contribution.PublicKeys.Alpha.XR) {
		return errors.New("couldn't verify that [α]₁ is based on previous contribution")
	}
	if !sameRatio(contribution.Parameters.G1.BetaTau[0], current.Parameters.G1.BetaTau[0], betaR, contribution.PublicKeys.Beta.XR) {
		return errors.New("couldn't verify that [β]₁ is based on previous contribution")
	}
	if !sameRatio(contribution.PublicKeys.Tau.SG, contribution.PublicKeys.Tau.SXG, contribution.Parameters.G2.Tau[1], current.Parameters.G2.Tau[1]) {
		return errors.New("couldn't verify that [τ]₂ is based on previous contribution")
	}
	if !sameRatio(contribution.PublicKeys.Beta.SG, contribution.PublicKeys.Beta.SXG, contribution.Parameters.G2.Beta, current.Parameters.G2.Beta) {
		return errors.New("couldn't verify that [β]₂ is based on previous contribution")
	}

	// Check for valid updates using powers of τ
	_, _, g1, g2 := curve.Generators()
	tauL1, tauL2 := linearCombinationG1(contribution.Parameters.G1.Tau)
	if !sameRatio(tauL1, tauL2, contribution.Parameters.G2.Tau[1], g2) {
		return errors.New("couldn't verify valid powers of τ in G₁")
	}
	alphaL1, alphaL2 := linearCombinationG1(contribution.Parameters.G1.AlphaTau)
	if !sameRatio(alphaL1, alphaL2, contribution.Parameters.G2.Tau[1], g2) {
		return errors.New("couldn't verify valid powers of α(τ) in G₁")
	}
	betaL1, betaL2 := linearCombinationG1(contribution.Parameters.G1.BetaTau)
	if !sameRatio(betaL1, betaL2, contribution.Parameters.G2.Tau[1], g2) {
		return errors.New("couldn't verify valid powers of α(τ) in G₁")
	}
	tau2L1, tau2L2 := linearCombinationG2(contribution.Parameters.G2.Tau)
	if !sameRatio(contribution.Parameters.G1.Tau[1], g1, tau2L1, tau2L2) {
		return errors.New("couldn't verify valid powers of τ in G₂")
	}

	// Check hash of the contribution
	h := contribution.hash()
	for i := 0; i < len(h); i++ {
		if h[i] != contribution.Hash[i] {
			return errors.New("couldn't verify hash of contribution")
		}
	}

	return nil
}

func (phase1 *Phase1) hash() []byte {
	sha := sha256.New()
	phase1.writeTo(sha)
	return sha.Sum(nil)
}
