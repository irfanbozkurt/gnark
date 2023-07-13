package polynomial

import (
	"math/bits"

	"github.com/consensys/gnark/frontend"
)

type Polynomial []frontend.Variable
type MultiLin []frontend.Variable

// Evaluate assumes len(m) = 1 << len(at)
func (m MultiLin) Evaluate(api frontend.API, at []frontend.Variable) frontend.Variable {

	eqs := make([]frontend.Variable, len(m))
	eqs[0] = 1
	for i, rI := range at {
		prevSize := 1 << i
		for j := prevSize - 1; j >= 0; j-- {
			eqs[2*j+1] = api.Mul(rI, eqs[j])
			eqs[2*j] = api.Sub(eqs[j], eqs[2*j+1]) // eq[2j] == (1 - rI) * eq[j]
		}
	}

	evaluation := frontend.Variable(0)
	for j := range m {
		evaluation = api.MulAcc(evaluation, eqs[j], m[j])
	}
	return evaluation
}

func (m MultiLin) NumVars() int {
	return bits.TrailingZeros(uint(len(m)))
}

func (p Polynomial) Eval(api frontend.API, at frontend.Variable) (pAt frontend.Variable) {
	pAt = 0

	for i := len(p) - 1; i >= 0; i-- {
		pAt = api.Add(pAt, p[i])
		if i != 0 {
			pAt = api.Mul(pAt, at)
		}
	}

	return
}

// negFactorial returns (-n)(-n+1)...(-2)(-1)
// There are more efficient algorithms, but we are talking small values here so it doesn't matter
func negFactorial(n int) int {
	n = -n
	result := n
	for n++; n <= -1; n++ {
		result *= n
	}
	return result
}

// computeDeltaAtNaive brute forces the computation of the δᵢ(@), which are the lagrange bases, evaluated at @
// TODO @Tabaie consider computing these symbolically (as polynomials in @) at compile time if it improves proving time
func computeDeltaAtNaive(api frontend.API, at frontend.Variable, valuesLen int) []frontend.Variable {
	deltaAt := make([]frontend.Variable, valuesLen)
	atMinus := make([]frontend.Variable, valuesLen) //TODO: No need for this array and the following loop
	toPrint := make([]frontend.Variable, valuesLen+1)
	// atMinusᵢ = @ - i
	for i := range atMinus {
		atMinus[i] = api.Sub(at, i)
	}
	toPrint[0] = "atMinus"
	copy(toPrint[1:], atMinus)
	api.Println(toPrint...)

	// initially each δᵢ(@) is computed as ∏_{j ≠ i} @-j which ensures that the lagrange basis zeros out at all points except i
	// that gives δ_i(i) = i × i-1 × ... 1 × -1 × ... × i-(n-1); we need to divide out this value (call it the "normalizer")
	// normalizer₀ = -1 × -2 × ... × -n+1
	normalizerInv := api.Inverse(negFactorial(valuesLen - 1))

	//api.Println("normalizerInv", 0, normalizerInv)

	for i := range deltaAt {
		deltaAt[i] = normalizerInv
		for j := range atMinus {
			if i != j {
				if i == 5 && j == 3 {
					api.Println("atMinus", j, "=", atMinus[j])
					api.Println(deltaAt[i], "×", atMinus[j], "=", api.Mul(deltaAt[i], atMinus[j]))
				}

				if i == 5 && j == 3 {
					deltaAt[i] = api.Mul(deltaAt[i], atMinus[j])
					api.Println("deltaAt 5,", j, "=", deltaAt[i])
				} else {
					deltaAt[i] = api.Mul(deltaAt[i], atMinus[j])
				}
			}
		}
		//api.Println("deltaAt", i, deltaAt[i])

		if i+1 < len(deltaAt) {
			normalizerInvAdjustment := api.DivUnchecked(i+1-valuesLen, i+1) // normalizer ᵢ₊₁ = normalizer ᵢ × (i+1)/[i-(n-1)]
			normalizerInv = api.Mul(normalizerInvAdjustment, normalizerInv)
			//api.Println("normalizerInv", i+1, normalizerInv)
		}
	}
	return deltaAt
}

// InterpolateLDE fits a polynomial f of degree len(values)-1 such that f(i) = values[i] whenever defined. Returns f(at)
func InterpolateLDE(api frontend.API, at frontend.Variable, values []frontend.Variable) frontend.Variable {
	deltaAt := computeDeltaAtNaive(api, at, len(values))
	toPrint := make([]frontend.Variable, 1, len(deltaAt)+1)
	toPrint[0] = "delta at"
	toPrint = append(toPrint, deltaAt...)
	api.Println(toPrint...)

	res := frontend.Variable(0)

	for i, c := range values {
		res = api.MulAcc(res, c, deltaAt[i])
	}

	return res
}

// EvalEq returns Πⁿ₁ Eq(xᵢ, yᵢ) = Πⁿ₁ xᵢyᵢ + (1-xᵢ)(1-yᵢ) = Πⁿ₁ (1 + 2xᵢyᵢ - xᵢ - yᵢ). Is assumes len(x) = len(y) =: n
func EvalEq(api frontend.API, x, y []frontend.Variable) (eq frontend.Variable) {

	eq = 1
	for i := range x {
		next := api.Mul(x[i], y[i])
		next = api.Add(next, next)
		next = api.Add(next, 1)
		next = api.Sub(next, x[i])
		next = api.Sub(next, y[i])

		eq = api.Mul(eq, next)
	}
	return
}
