package iotago

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2"

	"golang.org/x/crypto/blake2b"
)

const (
	// TreasuryInputBytesLength is the length of a TreasuryInput.
	TreasuryInputBytesLength = blake2b.Size256
	// TreasuryInputSerializedBytesSize is the size of a serialized TreasuryInput with its type denoting byte.
	TreasuryInputSerializedBytesSize = serializer.SmallTypeDenotationByteSize + TreasuryInputBytesLength
)

// TreasuryInput is an input which references a milestone which generated a TreasuryOutput.
type TreasuryInput [32]byte

func (ti *TreasuryInput) Type() InputType {
	return InputTreasury
}

func (ti *TreasuryInput) Clone() *TreasuryInput {
	p := *ti
	return &p
}

func (ti *TreasuryInput) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
		if err := serializer.CheckMinByteLength(TreasuryInputSerializedBytesSize, len(data)); err != nil {
			return 0, fmt.Errorf("invalid treasury input bytes: %w", err)
		}
		if err := serializer.CheckTypeByte(data, byte(InputTreasury)); err != nil {
			return 0, fmt.Errorf("unable to deserialize treasury input: %w", err)
		}
	}
	copy(ti[:], data[serializer.SmallTypeDenotationByteSize:])
	return TreasuryInputSerializedBytesSize, nil
}

func (ti *TreasuryInput) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (data []byte, err error) {
	var b [TreasuryInputSerializedBytesSize]byte
	b[0] = byte(InputTreasury)
	copy(b[serializer.SmallTypeDenotationByteSize:], ti[:])
	return b[:], nil
}

func (ti *TreasuryInput) Size() int {
	return TreasuryInputSerializedBytesSize
}

func (ti *TreasuryInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonTreasuryInput{
		Type:        int(InputTreasury),
		MilestoneID: EncodeHex(ti[:]),
	})
}

func (ti *TreasuryInput) UnmarshalJSON(bytes []byte) error {
	jTreasuryInput := &jsonTreasuryInput{}
	if err := json.Unmarshal(bytes, jTreasuryInput); err != nil {
		return err
	}
	seri, err := jTreasuryInput.ToSerializable()
	if err != nil {
		return err
	}
	*ti = *seri.(*TreasuryInput)
	return nil
}

// jsonTreasuryInput defines the json representation of a TreasuryInput.
type jsonTreasuryInput struct {
	Type        int    `json:"type"`
	MilestoneID string `json:"milestoneId"`
}

func (j *jsonTreasuryInput) ToSerializable() (serializer.Serializable, error) {
	msHash, err := DecodeHex(j.MilestoneID)
	if err != nil {
		return nil, fmt.Errorf("unable to decode milestone hash from JSON for treasury input: %w", err)
	}
	if err := serializer.CheckExactByteLength(len(msHash), MilestoneIDLength); err != nil {
		return nil, fmt.Errorf("unable to decode milestone hash from JSON for treasury input: %w", err)
	}
	input := &TreasuryInput{}
	copy(input[:], msHash)
	return input, nil
}