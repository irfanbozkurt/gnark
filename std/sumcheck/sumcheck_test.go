package sumcheck

import (
	"fmt"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/polynomial"
	"github.com/consensys/gnark/test"
	"math/bits"
	"testing"
)

type singleMultilinLazyClaim struct {
	g          polynomial.MultiLin
	claimedSum frontend.Variable
}

type singleMultilinProof struct {
	PartialSumPolys [][]frontend.Variable
}

func (p singleMultilinProof) PartialSumPoly(index int) polynomial.Polynomial {
	return p.PartialSumPolys[index]
}

func (s singleMultilinProof) FinalEvalProof() Proof {
	return nil
}

func (c singleMultilinLazyClaim) ClaimsNum() int {
	return 1
}

func (c singleMultilinLazyClaim) VarsNum() int {
	return bits.TrailingZeros(uint(len(c.g)))
}

func (c singleMultilinLazyClaim) CombinedSum(frontend.Variable) frontend.Variable {
	return c.claimedSum
}

func (c singleMultilinLazyClaim) Degree(int) int {
	return 1
}

func (c singleMultilinLazyClaim) VerifyFinalEval(api frontend.API, r []frontend.Variable, _, purportedValue frontend.Variable, _ interface{}) error {
	val := c.g.Eval(api, r)
	api.AssertIsEqual(val, purportedValue)
	return nil
}

func sumAsInts(poly polynomial.MultiLin) (sum int) {
	sum = 0
	for _, i := range poly {
		sum += i.(int)
	}
	return
}

func testSumcheckSingleClaimMultilin(t *testing.T, poly polynomial.MultiLin, proof Proof, transcript ArithmeticTranscript) {
	verifier := Verifier{
		Claims:     singleMultilinLazyClaim{g: poly, claimedSum: sumAsInts(poly)},
		Proof:      proof,
		Transcript: transcript,
	}

	assert := test.NewAssert(t)

	assert.SolvingSucceeded(&verifier, &verifier)

}

func TestSumcheckSingleClaimMultilin(t *testing.T) {
	testSumcheckSingleClaimMultilin(
		t,
		polynomial.MultiLin{1, 2, 3, 4},
		singleMultilinProof{
			PartialSumPolys: [][]frontend.Variable{{7}, {2}},
		},
		NewMessageCounter(-2, 1),
	)
}

// MessageCounter is a very bad fiat-shamir challenge generator
type MessageCounter struct {
	state   uint64
	step    uint64
	updated bool
}

func (m *MessageCounter) Update(...interface{}) {
	m.state += m.step
	m.updated = true
}

func (m *MessageCounter) Next(i ...interface{}) frontend.Variable {
	if !m.updated || len(i) != 0 {
		m.Update(i)
	}
	fmt.Println("hash returning", m.state)
	m.updated = false
	return m.state
}

func (m *MessageCounter) NextN(N int, i ...interface{}) (challenges []frontend.Variable) {
	challenges = make([]frontend.Variable, N)
	for n := 0; n < N; n++ {
		challenges[n] = m.Next(i)
		i = []interface{}{}
	}
	return
}

func NewMessageCounter(startState, step int) ArithmeticTranscript {
	transcript := &MessageCounter{state: uint64(startState), step: uint64(step)}
	transcript.Update([]byte{})
	return transcript
}

func NewMessageCounterGenerator(startState, step int) func() ArithmeticTranscript {
	return func() ArithmeticTranscript {
		return NewMessageCounter(startState, step)
	}
}