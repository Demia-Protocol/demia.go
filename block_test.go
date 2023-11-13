package iotago_test

import (
	"crypto/ed25519"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	hiveEd25519 "github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/api"
	"github.com/iotaledger/iota.go/v4/builder"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/iota.go/v4/tpkg"
)

func TestBlock_DeSerialize(t *testing.T) {
	tests := []deSerializeTest{
		{
			name:   "ok - no payload",
			source: tpkg.RandBlock(tpkg.RandBasicBlock(tpkg.TestAPI, 255), tpkg.TestAPI, 0),
			target: &iotago.Block{},
		},
		{
			name:   "ok - transaction",
			source: tpkg.RandBlock(tpkg.RandBasicBlock(tpkg.TestAPI, iotago.PayloadSignedTransaction), tpkg.TestAPI, 0),
			target: &iotago.Block{},
		},
		{
			name:   "ok - tagged data",
			source: tpkg.RandBlock(tpkg.RandBasicBlock(tpkg.TestAPI, iotago.PayloadTaggedData), tpkg.TestAPI, 0),
			target: &iotago.Block{},
		},
		{
			name:   "ok - validation block",
			source: tpkg.RandBlock(tpkg.RandValidationBlock(tpkg.TestAPI), tpkg.TestAPI, 0),
			target: &iotago.Block{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.deSerialize)
	}
}

func createBlockWithParents(t *testing.T, strongParents, weakParents, shallowLikeParent iotago.BlockIDs, apiProvider *api.EpochBasedProvider) error {
	t.Helper()

	apiForSlot := apiProvider.LatestAPI()

	block, err := builder.NewBasicBlockBuilder(apiForSlot).
		StrongParents(strongParents).
		WeakParents(weakParents).
		ShallowLikeParents(shallowLikeParent).
		IssuingTime(time.Now()).
		SlotCommitmentID(iotago.NewCommitment(apiForSlot.Version(), apiForSlot.TimeProvider().SlotFromTime(time.Now())-apiForSlot.ProtocolParameters().MinCommittableAge(), iotago.CommitmentID{}, iotago.Identifier{}, 0, 0).MustID()).
		Build()
	require.NoError(t, err)

	return lo.Return2(apiForSlot.Encode(block, serix.WithValidation()))
}

func createBlockAtSlot(t *testing.T, blockIndex, commitmentIndex iotago.SlotIndex, apiProvider *api.EpochBasedProvider) error {
	t.Helper()

	apiForSlot := apiProvider.APIForSlot(blockIndex)

	block, err := builder.NewBasicBlockBuilder(apiForSlot).
		StrongParents(iotago.BlockIDs{tpkg.RandBlockID()}).
		IssuingTime(apiForSlot.TimeProvider().SlotStartTime(blockIndex)).
		SlotCommitmentID(iotago.NewCommitment(apiForSlot.Version(), commitmentIndex, iotago.CommitmentID{}, iotago.Identifier{}, 0, 0).MustID()).
		Build()
	require.NoError(t, err)

	return lo.Return2(apiForSlot.Encode(block, serix.WithValidation()))
}

func createBlockAtSlotWithVersion(t *testing.T, blockIndex iotago.SlotIndex, version iotago.Version, apiProvider *api.EpochBasedProvider) error {
	t.Helper()

	apiForSlot := apiProvider.APIForSlot(blockIndex)
	block, err := builder.NewBasicBlockBuilder(apiForSlot).
		ProtocolVersion(version).
		StrongParents(iotago.BlockIDs{iotago.BlockID{}}).
		IssuingTime(apiForSlot.TimeProvider().SlotStartTime(blockIndex)).
		SlotCommitmentID(iotago.NewCommitment(apiForSlot.Version(), blockIndex-apiForSlot.ProtocolParameters().MinCommittableAge(), iotago.CommitmentID{}, iotago.Identifier{}, 0, 0).MustID()).
		Build()
	require.NoError(t, err)

	return lo.Return2(apiForSlot.Encode(block, serix.WithValidation()))
}

//nolint:unparam // in the test we always issue at blockIndex=100, but let's keep this flexibility.
func createBlockAtSlotWithPayload(t *testing.T, blockIndex, commitmentIndex iotago.SlotIndex, payload iotago.ApplicationPayload, apiProvider *api.EpochBasedProvider) error {
	t.Helper()

	apiForSlot := apiProvider.APIForSlot(blockIndex)

	block, err := builder.NewBasicBlockBuilder(apiForSlot).
		StrongParents(iotago.BlockIDs{tpkg.RandBlockID()}).
		IssuingTime(apiForSlot.TimeProvider().SlotStartTime(blockIndex)).
		SlotCommitmentID(iotago.NewCommitment(apiForSlot.Version(), commitmentIndex, iotago.CommitmentID{}, iotago.Identifier{}, 0, 0).MustID()).
		Payload(payload).
		Build()
	require.NoError(t, err)

	return lo.Return2(apiForSlot.Encode(block, serix.WithValidation()))
}

func TestBlock_ProtocolVersionSyntactical(t *testing.T) {
	apiProvider := api.NewEpochBasedProvider(
		api.WithAPIForMissingVersionCallback(
			func(parameters iotago.ProtocolParameters) (iotago.API, error) {
				return iotago.V3API(iotago.NewV3ProtocolParameters(iotago.WithVersion(parameters.Version()))), nil
			},
		),
	)
	apiProvider.AddProtocolParametersAtEpoch(iotago.NewV3ProtocolParameters(), 0)
	apiProvider.AddProtocolParametersAtEpoch(iotago.NewV3ProtocolParameters(iotago.WithVersion(4)), 3)

	timeProvider := apiProvider.CommittedAPI().TimeProvider()

	require.ErrorIs(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(1), 2, apiProvider), iotago.ErrInvalidBlockVersion)

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochEnd(1), 3, apiProvider))

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochEnd(2), 3, apiProvider))

	require.ErrorIs(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(3), 3, apiProvider), iotago.ErrInvalidBlockVersion)

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(3), 4, apiProvider))

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochEnd(3), 4, apiProvider))

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(5), 4, apiProvider))

	apiProvider.AddProtocolParametersAtEpoch(iotago.NewV3ProtocolParameters(iotago.WithVersion(5)), 10)

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochEnd(9), 4, apiProvider))

	require.ErrorIs(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(10), 4, apiProvider), iotago.ErrInvalidBlockVersion)

	require.NoError(t, createBlockAtSlotWithVersion(t, timeProvider.EpochStart(10), 5, apiProvider))
}

func TestBlock_Commitments(t *testing.T) {
	// with the following parameters, a block issued in slot 100 can commit between slot 80 and 90
	apiProvider := api.NewEpochBasedProvider()
	apiProvider.AddProtocolParametersAtEpoch(
		iotago.NewV3ProtocolParameters(
			iotago.WithTimeOptions(0, time.Now().Add(-20*time.Minute).Unix(), 10, 13, 15, 30, 11, 21, 24),
		), 0)

	require.ErrorIs(t, createBlockAtSlot(t, 100, 78, apiProvider), iotago.ErrCommitmentTooOld)

	require.ErrorIs(t, createBlockAtSlot(t, 100, 90, apiProvider), iotago.ErrCommitmentTooRecent)

	require.NoError(t, createBlockAtSlot(t, 100, 89, apiProvider))

	require.NoError(t, createBlockAtSlot(t, 100, 80, apiProvider))

	require.NoError(t, createBlockAtSlot(t, 100, 85, apiProvider))
}

func TestBlock_Commitments1(t *testing.T) {
	// with the following parameters, a block issued in slot 100 can commit between slot 80 and 90
	apiProvider := api.NewEpochBasedProvider()
	apiProvider.AddProtocolParametersAtEpoch(
		iotago.NewV3ProtocolParameters(
			iotago.WithTimeOptions(0, time.Now().Add(-20*time.Minute).Unix(), 10, 13, 15, 30, 7, 21, 24),
		), 0)

	require.ErrorIs(t, createBlockAtSlot(t, 10, 4, apiProvider), iotago.ErrCommitmentTooRecent)

}

func TestBlock_TransactionCreationTime(t *testing.T) {
	keyPair := hiveEd25519.GenerateKeyPair()
	// We derive a dummy account from addr.
	addr := iotago.Ed25519AddressFromPubKey(keyPair.PublicKey[:])
	output := &iotago.BasicOutput{
		Amount: 100000,
		UnlockConditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: addr,
			},
		},
	}
	// with the following parameters, block issued in slot 110 can contain a transaction with commitment input referencing
	// commitments between 90 and slot that the block commits to (100 at most)
	apiProvider := api.NewEpochBasedProvider()
	apiProvider.AddProtocolParametersAtEpoch(
		iotago.NewV3ProtocolParameters(
			iotago.WithTimeOptions(0, time.Now().Add(-20*time.Minute).Unix(), 10, 13, 15, 30, 7, 21, 24),
		), 0)

	creationSlotTooRecent, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		SetCreationSlot(101).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(78, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.ErrorIs(t, createBlockAtSlotWithPayload(t, 100, 79, creationSlotTooRecent, apiProvider), iotago.ErrTransactionCreationSlotTooRecent)

	creationSlotCorrectEqual, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		SetCreationSlot(100).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, creationSlotCorrectEqual, apiProvider))

	creationSlotCorrectSmallerThanCommitment, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		SetCreationSlot(1).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, creationSlotCorrectSmallerThanCommitment, apiProvider))

	creationSlotCorrectLargerThanCommitment, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		SetCreationSlot(99).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, creationSlotCorrectLargerThanCommitment, apiProvider))
}

func TestBlock_WeakParents(t *testing.T) {
	// with the following parameters, a block issued in slot 100 can commit between slot 80 and 90
	apiProvider := api.NewEpochBasedProvider()
	apiProvider.AddProtocolParametersAtEpoch(
		iotago.NewV3ProtocolParameters(
			iotago.WithTimeOptions(0, time.Now().Add(-20*time.Minute).Unix(), 10, 13, 15, 30, 10, 20, 24),
		), 0)
	strongParent1 := tpkg.RandBlockID()
	strongParent2 := tpkg.RandBlockID()
	weakParent1 := tpkg.RandBlockID()
	weakParent2 := tpkg.RandBlockID()
	shallowLikeParent1 := tpkg.RandBlockID()
	shallowLikeParent2 := tpkg.RandBlockID()
	require.ErrorIs(t, createBlockWithParents(
		t,
		iotago.BlockIDs{strongParent1, strongParent2},
		iotago.BlockIDs{weakParent1, weakParent2, shallowLikeParent2},
		iotago.BlockIDs{shallowLikeParent1, shallowLikeParent2},
		apiProvider,
	), iotago.ErrWeakParentsInvalid)

	require.ErrorIs(t, createBlockWithParents(
		t,
		iotago.BlockIDs{strongParent1, strongParent2},
		iotago.BlockIDs{weakParent1, weakParent2, strongParent2},
		iotago.BlockIDs{shallowLikeParent1, shallowLikeParent2},
		apiProvider,
	), iotago.ErrWeakParentsInvalid)

	require.NoError(t, createBlockWithParents(
		t,
		iotago.BlockIDs{strongParent1, strongParent2},
		iotago.BlockIDs{weakParent1, weakParent2},
		iotago.BlockIDs{shallowLikeParent1, shallowLikeParent2},
		apiProvider,
	))

	require.NoError(t, createBlockWithParents(
		t,
		iotago.BlockIDs{strongParent1, strongParent2},
		iotago.BlockIDs{weakParent1, weakParent2},
		iotago.BlockIDs{shallowLikeParent1, shallowLikeParent2, strongParent2},
		apiProvider,
	))
}

func TestBlock_TransactionCommitmentInput(t *testing.T) {
	keyPair := hiveEd25519.GenerateKeyPair()
	// We derive a dummy account from addr.
	addr := iotago.Ed25519AddressFromPubKey(keyPair.PublicKey[:])
	output := &iotago.BasicOutput{
		Amount: 100000,
		UnlockConditions: iotago.BasicOutputUnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: addr,
			},
		},
	}
	// with the following parameters, block issued in slot 110 can contain a transaction with commitment input referencing
	// commitments between 90 and slot that the block commits to (100 at most)
	apiProvider := api.NewEpochBasedProvider()
	apiProvider.AddProtocolParametersAtEpoch(
		iotago.NewV3ProtocolParameters(
			iotago.WithTimeOptions(0, time.Now().Add(-20*time.Minute).Unix(), 10, 13, 15, 30, 11, 21, 24),
		), 0)

	commitmentInputTooOld, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(78, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.ErrorIs(t, createBlockAtSlotWithPayload(t, 100, 79, commitmentInputTooOld, apiProvider), iotago.ErrCommitmentInputTooOld)

	commitmentInputTooRecent, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(90, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.ErrorIs(t, createBlockAtSlotWithPayload(t, 100, 89, commitmentInputTooRecent, apiProvider), iotago.ErrCommitmentInputTooRecent)

	commitmentInputNewerThanBlockCommitment, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(85, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.ErrorIs(t, createBlockAtSlotWithPayload(t, 100, 79, commitmentInputNewerThanBlockCommitment, apiProvider), iotago.ErrCommitmentInputNewerThanCommitment)

	commitmentCorrect, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(79, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, commitmentCorrect, apiProvider))

	commitmentCorrectOldest, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(79, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 79, commitmentCorrectOldest, apiProvider))

	commitmentCorrectNewest, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(89, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, commitmentCorrectNewest, apiProvider))

	commitmentCorrectMiddle, err := builder.NewTransactionBuilder(apiProvider.LatestAPI()).
		AddInput(&builder.TxInput{
			UnlockTarget: addr,
			InputID:      tpkg.RandOutputID(0),
			Input:        output,
		}).
		AddOutput(output).
		AddContextInput(&iotago.CommitmentInput{CommitmentID: iotago.NewCommitmentID(85, tpkg.Rand32ByteArray())}).
		Build(iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: addr, Keys: ed25519.PrivateKey(keyPair.PrivateKey[:])}))

	require.NoError(t, err)

	require.NoError(t, createBlockAtSlotWithPayload(t, 100, 89, commitmentCorrectMiddle, apiProvider))
}

func TestBlock_DeserializationNotEnoughData(t *testing.T) {
	blockBytes := []byte{byte(tpkg.TestAPI.Version()), 1}

	block := &iotago.Block{}
	_, err := tpkg.TestAPI.Decode(blockBytes, block)
	require.ErrorIs(t, err, serializer.ErrDeserializationNotEnoughData)
}

func TestBasicBlock_MinSize(t *testing.T) {
	minBlock := &iotago.Block{
		API: tpkg.TestAPI,
		Header: iotago.BlockHeader{
			ProtocolVersion:  tpkg.TestAPI.Version(),
			IssuingTime:      tpkg.RandUTCTime(),
			SlotCommitmentID: iotago.NewEmptyCommitment(tpkg.TestAPI).MustID(),
		},
		Signature: tpkg.RandEd25519Signature(),
		Body: &iotago.BasicBlockBody{
			API:                tpkg.TestAPI,
			StrongParents:      tpkg.SortedRandBlockIDs(1),
			WeakParents:        iotago.BlockIDs{},
			ShallowLikeParents: iotago.BlockIDs{},
			Payload:            nil,
		},
	}

	blockBytes, err := tpkg.TestAPI.Encode(minBlock)
	require.NoError(t, err)

	block2 := &iotago.Block{}
	consumedBytes, err := tpkg.TestAPI.Decode(blockBytes, block2, serix.WithValidation())
	require.NoError(t, err)
	require.Equal(t, minBlock, block2)
	require.Equal(t, len(blockBytes), consumedBytes)
}

func TestValidationBlock_MinSize(t *testing.T) {
	minBlock := &iotago.Block{
		API: tpkg.TestAPI,
		Header: iotago.BlockHeader{
			ProtocolVersion:  tpkg.TestAPI.Version(),
			IssuingTime:      tpkg.RandUTCTime(),
			SlotCommitmentID: iotago.NewEmptyCommitment(tpkg.TestAPI).MustID(),
		},
		Signature: tpkg.RandEd25519Signature(),
		Body: &iotago.ValidationBlockBody{
			API:                     tpkg.TestAPI,
			StrongParents:           tpkg.SortedRandBlockIDs(1),
			WeakParents:             iotago.BlockIDs{},
			ShallowLikeParents:      iotago.BlockIDs{},
			HighestSupportedVersion: tpkg.TestAPI.Version(),
		},
	}

	blockBytes, err := tpkg.TestAPI.Encode(minBlock)
	require.NoError(t, err)

	block2 := &iotago.Block{}
	consumedBytes, err := tpkg.TestAPI.Decode(blockBytes, block2, serix.WithValidation())
	require.NoError(t, err)
	require.Equal(t, minBlock, block2)
	require.Equal(t, len(blockBytes), consumedBytes)
}

func TestValidationBlock_HighestSupportedVersion(t *testing.T) {
	block := &iotago.Block{
		API: tpkg.TestAPI,
		Header: iotago.BlockHeader{
			ProtocolVersion:  tpkg.TestAPI.Version(),
			IssuingTime:      tpkg.RandUTCTime(),
			SlotCommitmentID: iotago.NewEmptyCommitment(tpkg.TestAPI).MustID(),
		},
		Signature: tpkg.RandEd25519Signature(),
	}

	// Invalid HighestSupportedVersion.
	{
		block.Body = &iotago.ValidationBlockBody{
			API:                     tpkg.TestAPI,
			StrongParents:           tpkg.SortedRandBlockIDs(1),
			WeakParents:             iotago.BlockIDs{},
			ShallowLikeParents:      iotago.BlockIDs{},
			HighestSupportedVersion: tpkg.TestAPI.Version() - 1,
		}
		blockBytes, err := tpkg.TestAPI.Encode(block)
		require.NoError(t, err)

		block2 := &iotago.Block{}
		_, err = tpkg.TestAPI.Decode(blockBytes, block2, serix.WithValidation())
		require.ErrorContains(t, err, "highest supported version")
	}

	// Valid HighestSupportedVersion.
	{
		block.Body = &iotago.ValidationBlockBody{
			API:                     tpkg.TestAPI,
			StrongParents:           tpkg.SortedRandBlockIDs(1),
			WeakParents:             iotago.BlockIDs{},
			ShallowLikeParents:      iotago.BlockIDs{},
			HighestSupportedVersion: tpkg.TestAPI.Version(),
		}
		blockBytes, err := tpkg.TestAPI.Encode(block)
		require.NoError(t, err)

		block2 := &iotago.Block{}
		consumedBytes, err := tpkg.TestAPI.Decode(blockBytes, block2, serix.WithValidation())
		require.NoError(t, err)
		require.Equal(t, block, block2)
		require.Equal(t, len(blockBytes), consumedBytes)
	}
}

func TestBlockJSONMarshalling(t *testing.T) {
	networkID := iotago.NetworkIDFromString("xxxNetwork")
	issuingTime := tpkg.RandUTCTime()
	commitmentID := iotago.NewEmptyCommitment(tpkg.TestAPI).MustID()
	issuerID := tpkg.RandAccountID()
	signature := tpkg.RandEd25519Signature()
	strongParents := tpkg.SortedRandBlockIDs(1)
	validationBlock := &iotago.Block{
		API: tpkg.TestAPI,
		Header: iotago.BlockHeader{
			ProtocolVersion:  tpkg.TestAPI.Version(),
			IssuingTime:      issuingTime,
			IssuerID:         issuerID,
			NetworkID:        networkID,
			SlotCommitmentID: commitmentID,
		},
		Body: &iotago.ValidationBlockBody{
			API:                     tpkg.TestAPI,
			StrongParents:           strongParents,
			HighestSupportedVersion: tpkg.TestAPI.Version(),
		},
		Signature: signature,
	}

	blockJSON := fmt.Sprintf(`{"header":{"protocolVersion":%d,"networkId":"%d","issuingTime":"%s","slotCommitmentId":"%s","latestFinalizedSlot":0,"issuerId":"%s"},"body":{"type":%d,"strongParents":["%s"],"weakParents":[],"shallowLikeParents":[],"highestSupportedVersion":%d,"protocolParametersHash":"0x0000000000000000000000000000000000000000000000000000000000000000"},"signature":{"type":%d,"publicKey":"%s","signature":"%s"}}`,
		tpkg.TestAPI.Version(),
		networkID,
		strconv.FormatUint(serializer.TimeToUint64(issuingTime), 10),
		commitmentID.ToHex(),
		issuerID.ToHex(),
		iotago.BlockBodyTypeValidation,
		strongParents[0].ToHex(),
		tpkg.TestAPI.Version(),
		iotago.SignatureEd25519,
		hexutil.EncodeHex(signature.PublicKey[:]),
		hexutil.EncodeHex(signature.Signature[:]),
	)

	jsonEncode, err := tpkg.TestAPI.JSONEncode(validationBlock)

	fmt.Println(string(jsonEncode))

	require.NoError(t, err)
	require.Equal(t, blockJSON, string(jsonEncode))
}

// TODO: add tests
//  - max size
//  - parents parameters basic block
//  - parents parameters validator block
//  - decode/encode protocol parameters
