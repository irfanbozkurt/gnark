package test

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"testing"
)

type mulCircuit struct {
	Factor1, Factor2, ExpectedProduct frontend.Variable
}

func (c *mulCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.ExpectedProduct, api.Mul(c.Factor1, c.Factor2))
	return nil
}

func TestMul(t *testing.T) {
	assert := NewAssert(t)
	assert.SolvingSucceeded(&mulCircuit{}, &mulCircuit{
		Factor1:         "10050209408565853175227483597402310118890689229267797249332367917488228060365",
		Factor2:         -4,
		ExpectedProduct: "12235037540862777778537806118576725362127795583456448825274187029985668943053"},
		WithCurves(ecc.BLS12_381))
}
