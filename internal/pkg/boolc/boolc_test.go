package boolc_test

import (
	"testing"

	"go-echo-boilerplate/internal/pkg/boolc"

	"github.com/stretchr/testify/assert"
)

func TestBoolc(t *testing.T) {
	cases := []struct {
		name                          string
		args                          []bool
		and, or, nand, nor, xor, xnor bool
	}{
		{"none", nil, true, false, false, true, false, true},
		{"one true", []bool{true}, true, true, false, false, true, false},
		{"one false", []bool{false}, false, false, true, true, false, true},
		{"true false", []bool{true, false}, false, true, true, false, true, false},
		{"two true", []bool{true, true}, true, true, false, false, false, true},
		{"three true", []bool{true, true, true}, true, true, false, false, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.and, boolc.And(tc.args...), "And")
			assert.Equal(t, tc.or, boolc.Or(tc.args...), "Or")
			assert.Equal(t, tc.nand, boolc.Nand(tc.args...), "Nand")
			assert.Equal(t, tc.nor, boolc.Nor(tc.args...), "Nor")
			assert.Equal(t, tc.xor, boolc.Xor(tc.args...), "Xor")
			assert.Equal(t, tc.xnor, boolc.Xnor(tc.args...), "Xnor")
		})
	}

	assert.False(t, boolc.Not(true))
	assert.True(t, boolc.Not(false))
}
