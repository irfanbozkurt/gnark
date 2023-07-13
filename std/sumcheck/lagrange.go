package sumcheck

import "github.com/consensys/gnark/frontend"

// negFactorial returns (-n)(-n+1)...(-2)(-1)
// There are more efficient algorithms, but we are talking small values here so it doesn't matter
func negFactorial(api frontend.API, n int) frontend.Variable {
	if n > 20 {
		// 20!           = 2432902008176640000
		// max abs int64 = 9223372036854775807
		panic("factorial too big") // TODO: Use the API
	}
	result := n
	n = -n
	for n++; n < -1; n++ {
		result *= n
	}
	return result
}

// InterpolateOnRange fits a polynomial f of degree len(values)-1 such that f(i) = values[i] whenever defined. Returns f(at)
// Algorithm taken from https://people.cs.georgetown.edu/jthaler/ProofsArgsAndZK.pdf section 2.4
// TODO @Tabaie this is more efficient than polynomial.InterpolateLDE but it is not "complete" (fails if 0 ≤ @ < n)
// TODO That wouldn't be a problem in real-world applications, but it is for tests, some of which use small, positive challenges
// TODO @Tabaie provide an option in the GKR and Sumcheck protocols to choose between the two
// TODO @Tabaie alternatively, when using small challenges, use negative ones so that the other impl is unnecessary
func InterpolateOnRange(api frontend.API, at frontend.Variable, values ...frontend.Variable) frontend.Variable {
	deltaAt := make([]frontend.Variable, len(values))
	deltaAt[0] = api.Inverse(negFactorial(api, len(values)-1))
	for k := 1; k < len(values); k++ {
		deltaAt[0] = api.Mul(deltaAt[0], api.Sub(at, k))
	}

	// Now recursively compute δᵢ(at) by noting it is equal to δᵢ(at) × (r-i+1) × (r-i)⁻¹ × i⁻¹ × (-len(values)+i)
	for i := 1; i < len(values); i++ {
		removeFromNumeratorAddToDenominator := api.Mul(i, api.Sub(at, i))
		removeFromDenominatorAddToNumerator := api.Mul(api.Sub(at, i-1), i-len(values))
		// TODO @Tabaie If we batch invert, we would get one extra constraint the prover will do fewer inversions
		adjustment := api.DivUnchecked(removeFromDenominatorAddToNumerator, removeFromNumeratorAddToDenominator) //TODO: May be shallower to mul removeFromDenominator and δᵢ₋₁ first and THEN divide
		deltaAt[i] = api.Mul(deltaAt[i-1], adjustment)
	}

	var res frontend.Variable
	res = 0

	for i, c := range values {
		res = api.Add(res,
			api.Mul(c, deltaAt[i]),
		)
	}

	return res
}
