package iotago_test

import (
	"testing"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/tpkg"
)

func TestTreasuryOutput_DeSerialize(t *testing.T) {
	tests := []deSerializeTest{
		{
			name:   "ok",
			source: tpkg.RandTreasuryOutput(),
			target: &iotago.TreasuryOutput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.deSerialize)
	}
}