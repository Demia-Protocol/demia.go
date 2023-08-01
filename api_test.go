//nolint:scopelint
package iotago_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/serializer/v2/serix"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

const (
	OneMi iotago.BaseToken = 1_000_000
)

type deSerializeTest struct {
	name      string
	source    any
	target    any
	seriErr   error
	deSeriErr error
}

func (test *deSerializeTest) deSerialize(t *testing.T) {
	serixData, err := tpkg.TestAPI.Encode(test.source, serix.WithValidation())
	if test.seriErr != nil {
		require.Error(t, err, test.seriErr)

		return
	}
	require.NoError(t, err)

	if src, ok := test.source.(iotago.Sizer); ok {
		require.Equal(t, src.Size(), len(serixData))
	}

	serixTarget := reflect.New(reflect.TypeOf(test.target).Elem()).Interface()
	bytesRead, err := tpkg.TestAPI.Decode(serixData, serixTarget)
	if test.deSeriErr != nil {
		require.Error(t, err, test.deSeriErr)

		return
	}
	require.NoError(t, err)
	require.Len(t, serixData, bytesRead)
	require.EqualValues(t, test.source, serixTarget)

	sourceJSON, err := tpkg.TestAPI.JSONEncode(test.source)
	require.NoError(t, err)

	jsonDest := reflect.New(reflect.TypeOf(test.target).Elem()).Interface()
	require.NoError(t, tpkg.TestAPI.JSONDecode(sourceJSON, jsonDest))

	require.EqualValues(t, test.source, jsonDest)
}

func TestProtocolParameters_DeSerialize(t *testing.T) {
	tests := []deSerializeTest{
		{
			name:   "ok",
			source: tpkg.RandProtocolParameters(),
			target: &iotago.V3ProtocolParameters{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.deSerialize)
	}
}

func TestProtocolParametersJSONMarshalling(t *testing.T) {
	var protoParams iotago.ProtocolParameters = iotago.NewV3ProtocolParameters(
		iotago.WithNetworkOptions(
			"xxxNetwork",
			"xxx",
		),
		iotago.WithSupplyOptions(
			1234567890987654321,
			6,
			7,
			8,
		),
		iotago.WithWorkScoreOptions(
			1,
			2,
			3,
			4,
			5,
			6,
			7,
			8,
			9,
			10,
			11,
			12,
		),
		iotago.WithTimeProviderOptions(
			1681373293,
			10,
			13,
		),
		iotago.WithManaOptions(
			1,
			27,
			[]uint32{10, 20},
			32,
			1337,
			20,
		),
		iotago.WithStakingOptions(11),
		iotago.WithLivenessOptions(
			3,
			10,
			20,
			24,
		),
		iotago.WithVersionSignalingOptions(3, 4, 1),
	)

	protoParamsJSON := `{"type":0,"version":3,"networkName":"xxxNetwork","bech32Hrp":"xxx","rentStructure":{"vByteCost":6,"vByteFactorData":7,"vByteFactorKey":8},"workScoreStructure":{"dataByte":1,"block":2,"missingParent":3,"input":4,"contextInput":5,"output":6,"nativeToken":7,"staking":8,"blockIssuer":9,"allotment":10,"signatureEd25519":11,"minStrongParentsThreshold":12},"tokenSupply":"1234567890987654321","genesisUnixTimestamp":"1681373293","slotDurationInSeconds":10,"slotsPerEpochExponent":13,"manaGenerationRate":1,"manaGenerationRateExponent":27,"manaDecayFactors":[10,20],"manaDecayFactorsExponent":32,"manaDecayFactorEpochsSum":1337,"manaDecayFactorEpochsSumExponent":20,"stakingUnbondingPeriod":"11","livenessThreshold":"3","minCommittableAge":"10","maxCommittableAge":"20","epochNearingThreshold":"24","versionSignaling":{"windowSize":3,"windowTargetRatio":4,"activationOffset":1}}`

	jsonProtoParams, err := tpkg.TestAPI.JSONEncode(protoParams)
	require.NoError(t, err)
	require.Equal(t, protoParamsJSON, string(jsonProtoParams))

	var decodedProtoParams iotago.ProtocolParameters
	err = tpkg.TestAPI.JSONDecode([]byte(protoParamsJSON), &decodedProtoParams)
	require.NoError(t, err)

	require.Equal(t, protoParams, decodedProtoParams)
}
