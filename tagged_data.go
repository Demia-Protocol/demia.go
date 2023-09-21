package iotago

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/iota.go/v3/util"
)

const (
	// TaggedPayloadTagMaxLength defines the max length of the tag within a TaggedData payload.
	TaggedPayloadTagMaxLength = 64
)

var (
	// ErrTaggedDataTagExceedsMaxSize gets returned when a TaggedData payload's tag exceeds TaggedPayloadTagMaxLength.
	ErrTaggedDataTagExceedsMaxSize = errors.New("tag exceeds max size")
)

// TaggedData is a payload which holds a tag and associated data.
type TaggedData struct {
	// The tag to use to categorize the data.
	Tag []byte
	// The data within the payload.
	Data []byte
	// Public key of the sender
	PublicKey []byte
	// Signature of the data bytes
	Signature []byte
}

func (u *TaggedData) PayloadType() PayloadType {
	return PayloadTaggedData
}

func (u *TaggedData) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	return serializer.NewDeserializer(data).
		CheckTypePrefix(uint32(PayloadTaggedData), serializer.TypeDenotationUint32, func(err error) error {
			return fmt.Errorf("unable to deserialize tagged data: %w", err)
		}).
		ReadVariableByteSlice(&u.Tag, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize tagged data tag: %w", err)
		}, 0, TaggedPayloadTagMaxLength).
		ReadVariableByteSlice(&u.Data, serializer.SeriLengthPrefixTypeAsUint32, func(err error) error {
			return fmt.Errorf("unable to deserialize tagged data data: %w", err)
		}, 0, BlockBinSerializedMaxSize). // obviously can never be that size
		ReadVariableByteSlice(&u.PublicKey, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize tagged data public key: %w", err)
		}, 0, 32).
		ReadVariableByteSlice(&u.Signature, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize tagged data signature: %w", err)
		}, 0, 64).
		Done()
}

func (u *TaggedData) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteNum(PayloadTaggedData, func(err error) error {
			return fmt.Errorf("unable to serialize tagged data payload ID: %w", err)
		}).
		WriteVariableByteSlice(u.Tag, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize tagged data tag: %w", err)
		}, 0, TaggedPayloadTagMaxLength).
		// we do not check the length of the data field as in any circumstance
		// the max size it can take up is dependent on how big the enclosing
		// parent object is
		WriteVariableByteSlice(u.Data, serializer.SeriLengthPrefixTypeAsUint32, func(err error) error {
			return fmt.Errorf("unable to serialize tagged data data: %w", err)
		}, 0, 0).
		WriteVariableByteSlice(u.PublicKey, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize tagged data public key: %w", err)
		}, 0, 32).
		WriteVariableByteSlice(u.Signature, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize tagged data signature: %w", err)
		}, 0, 64).
		Serialize()
}

func (u *TaggedData) Size() int {
	// length prefixes for tag and data  = 1 (uint8) and 4 (uint32)
	return util.NumByteLen(uint32(PayloadTaggedData)) +
		serializer.OneByte + len(u.Tag) +
		serializer.UInt32ByteSize + len(u.Data) +
		serializer.OneByte + len(u.PublicKey) +
		serializer.OneByte + len(u.Signature)
}

func (u *TaggedData) MarshalJSON() ([]byte, error) {
	jTaggedData := &jsonTaggedData{}
	jTaggedData.Type = int(PayloadTaggedData)
	jTaggedData.Tag = EncodeHex(u.Tag)
	jTaggedData.Data = EncodeHex(u.Data)
	jTaggedData.PublicKey = EncodeHex(u.PublicKey)
	jTaggedData.Signature = EncodeHex(u.Signature)
	return json.Marshal(jTaggedData)
}

func (u *TaggedData) UnmarshalJSON(bytes []byte) error {
	jTaggedData := &jsonTaggedData{}
	if err := json.Unmarshal(bytes, jTaggedData); err != nil {
		return err
	}
	seri, err := jTaggedData.ToSerializable()
	if err != nil {
		return err
	}
	*u = *seri.(*TaggedData)
	return nil
}

// jsonTaggedData defines the json representation of a TaggedData payload.
type jsonTaggedData struct {
	Type      int    `json:"type"`
	Tag       string `json:"tag,omitempty"`
	Data      string `json:"data,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func (j *jsonTaggedData) ToSerializable() (serializer.Serializable, error) {
	tagBytes, err := DecodeHex(j.Tag)
	if err != nil {
		return nil, fmt.Errorf("unable to decode tag from JSON for tagged data payload: %w", err)
	}

	dataBytes, err := DecodeHex(j.Data)
	if err != nil {
		return nil, fmt.Errorf("unable to decode data from JSON for tagged data payload: %w", err)
	}

	pkBytes, err := DecodeHex(j.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("unable to decode public key from JSON for tagged data payload: %w", err)
	}

	sigBytes, err := DecodeHex(j.Signature)
	if err != nil {
		return nil, fmt.Errorf("unable to decode signature from JSON for tagged data payload: %w", err)
	}

	return &TaggedData{Tag: tagBytes, Data: dataBytes, PublicKey: pkBytes, Signature: sigBytes}, nil
}
