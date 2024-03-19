package iotago_test

import (
	"encoding/json"
	"testing"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/iota.go/v3/tpkg"
	"github.com/stretchr/testify/assert"

	iotago "github.com/iotaledger/iota.go/v3"
)

func TestTaggedDataDeSerialize(t *testing.T) {
	const tag = "寿司を作って"

	tests := []deSerializeTest{
		{
			name:   "ok",
			source: tpkg.RandTaggedData([]byte(tag)),
			target: &iotago.TaggedData{},
		},
		{
			name:   "empty-tag",
			source: tpkg.RandTaggedData(nil),
			target: &iotago.TaggedData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.deSerialize)
	}
}

func TestTaggedDataCustom(t *testing.T) {
	data := `
		{
			"type": 5,
			"tag": "0x00",
			"data": "0x00",
			"signature": "0x00AA",
			"publicKey": "0x00BB"
		}`

	tagged := &iotago.TaggedData{}
	assert.NoError(t, json.Unmarshal([]byte(data), tagged))
	assert.Equal(t, 17, tagged.Size())

	bytes, _ := tagged.Serialize(serializer.DeSeriModePerformValidation, tpkg.TestProtoParas);

	new := &iotago.TaggedData{}
	new.Deserialize(bytes, serializer.DeSeriModePerformValidation, tpkg.TestProtoParas)

	assert.Equal(t, new, tagged)
	assert.Equal(t, new.Size(), tagged.Size())

}