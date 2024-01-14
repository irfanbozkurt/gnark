package main

import (
	goLzss "github.com/consensys/compress/lzss"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestE2E(t *testing.T) {
	circuit, assignment, err := getCircuits(goLzss.GoodCompression, 64)
	assert.NoError(t, err)
	test.NewAssert(t).CheckCircuit(&circuit, test.WithValidAssignment(&assignment), test.WithBackends(backend.PLONK), test.WithCurves(ecc.BLS12_377))
}
