package iotago

import (
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/ierrors"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/iota.go/v4/util"
)

const (
	// 	DelegationIDLength is the byte length of a DelegationID.
	DelegationIDLength = blake2b.Size256
)

var (
	// ErrInvalidDelegationTransition gets returned when a Delegation Output is doing an invalid state transition.
	ErrInvalidDelegationTransition = ierrors.New("invalid delegation output transition")
	// ErrInvalidDelegationRewardsClaiming gets returned when it is invalid to claim rewards from a delegation output.
	ErrInvalidDelegationRewardsClaiming = ierrors.New("invalid delegation mana rewards claiming")
	// ErrInvalidDelegationNonZeroedID gets returned when a delegation output's delegation ID is not zeroed initially.
	ErrInvalidDelegationNonZeroedID = ierrors.New("delegation ID must be zeroed initially")
	// ErrInvalidDelegationModified gets returned when a delegation output's immutable fields are modified.
	ErrInvalidDelegationModified = ierrors.New("delegated amount, validator ID and start epoch cannot be modified")
	// ErrInvalidDelegationStartEpoch gets returned when a delegation output's start epoch is not set correctly
	// relative to the slot of the current epoch in which the voting power is calculated.
	ErrInvalidDelegationStartEpoch = ierrors.New("invalid start epoch")
	// ErrInvalidDelegationAmount gets returned when a delegation output's delegated amount is not equal to the amount.
	ErrInvalidDelegationAmount = ierrors.New("delegated amount equal to the amount")
	// ErrInvalidDelegationNonZeroEndEpoch gets returned when a delegation output's end epoch is not zero at genesis.
	ErrInvalidDelegationNonZeroEndEpoch = ierrors.New("end epoch must be set to zero at output genesis")
	// ErrInvalidDelegationEndEpoch gets returned when a delegation output's end epoch is not set correctly
	// relative to the slot of the current epoch in which the voting power is calculated.
	ErrInvalidDelegationEndEpoch = ierrors.New("invalid end epoch")
	emptyDelegationID            = [DelegationIDLength]byte{}
)

func EmptyDelegationId() DelegationID {
	return emptyDelegationID
}

// DelegationID is the identifier for a Delegation Output.
// It is computed as the Blake2b-256 hash of the OutputID of the output which created the Delegation Output.
type DelegationID [DelegationIDLength]byte

// DelegationIDs are DelegationID(s).
type DelegationIDs []DelegationID

func (delegationId DelegationID) Addressable() bool {
	return false
}

func (delegationId DelegationID) Key() interface{} {
	return delegationId.String()
}

func (delegationId DelegationID) Empty() bool {
	return delegationId == emptyDelegationID
}

func (delegationId DelegationID) ToAddress() ChainAddress {
	panic("Delegation ID is not addressable")
}

func (delegationId DelegationID) Matches(other ChainID) bool {
	otherDelegationId, isDelegationId := other.(DelegationID)
	if !isDelegationId {
		return false
	}
	return delegationId == otherDelegationId
}

func (delegationId DelegationID) String() string {
	return hexutil.EncodeHex(delegationId[:])
}

func (delegationId DelegationID) ToHex() string {
	return hexutil.EncodeHex(delegationId[:])
}

func (id DelegationID) FromOutputID(outid OutputID) ChainID {
	return DelegationIDFromOutputID(outid)
}

// DelegationIDFromOutputID returns the DelegationID computed from a given OutputID.
func DelegationIDFromOutputID(outputID OutputID) DelegationID {
	return blake2b.Sum256(outputID[:])
}

type (
	delegationOutputUnlockCondition  interface{ UnlockCondition }
	delegationOutputImmFeature       interface{ Feature }
	DelegationOutputUnlockConditions = UnlockConditions[delegationOutputUnlockCondition]
	DelegationOutputImmFeatures      = Features[delegationOutputImmFeature]
)

// DelegationOutput is an output type used to implement delegation.
type DelegationOutput struct {
	// The amount of IOTA tokens held by the output.
	Amount BaseToken `serix:"0,mapKey=amount"`
	// The amount of IOTA tokens that were delegated when the output was created.
	DelegatedAmount BaseToken `serix:"1,mapKey=delegatedAmount"`
	// The identifier for this output.
	DelegationID DelegationID `serix:"2,mapKey=delegationId"`
	// The Account ID of the validator to which this output is delegating.
	ValidatorID AccountID `serix:"3,mapKey=validatorId"`
	// The index of the first epoch for which this output delegates.
	StartEpoch EpochIndex `serix:"4,mapKey=startEpoch"`
	// The index of the last epoch for which this output delegates.
	EndEpoch EpochIndex `serix:"5,mapKey=endEpoch"`
	// The unlock conditions on this output.
	Conditions DelegationOutputUnlockConditions `serix:"6,mapKey=unlockConditions,omitempty"`
	// The immutable feature on the output.
	ImmutableFeatures DelegationOutputImmFeatures `serix:"7,mapKey=immutableFeatures,omitempty"`
}

func (d *DelegationOutput) Clone() Output {
	return &DelegationOutput{
		Amount:            d.Amount,
		DelegatedAmount:   d.DelegatedAmount,
		DelegationID:      d.DelegationID,
		ValidatorID:       d.ValidatorID,
		StartEpoch:        d.StartEpoch,
		EndEpoch:          d.EndEpoch,
		Conditions:        d.Conditions.Clone(),
		ImmutableFeatures: d.ImmutableFeatures.Clone(),
	}
}

func (d *DelegationOutput) Ident() Address {
	return d.Conditions.MustSet().Address().Address
}

func (d *DelegationOutput) UnlockableBy(ident Address, txCreationTime SlotIndex) bool {
	ok, _ := outputUnlockable(d, nil, ident, txCreationTime)
	return ok
}

func (d *DelegationOutput) VBytes(rentStruct *RentStructure, _ VBytesFunc) VBytes {
	return outputOffsetVByteCost(rentStruct) +
		// type prefix + amount + delegated amount + start epoch + end epoch
		rentStruct.VBFactorData.Multiply(serializer.SmallTypeDenotationByteSize+serializer.UInt64ByteSize*4) +
		rentStruct.VBFactorData.Multiply(DelegationIDLength) +
		rentStruct.VBFactorData.Multiply(AccountIDLength) +
		d.Conditions.VBytes(rentStruct, nil) +
		d.ImmutableFeatures.VBytes(rentStruct, nil)
}

func (d *DelegationOutput) Chain() ChainID {
	return d.DelegationID
}

func (d *DelegationOutput) NativeTokenList() NativeTokens {
	return make(NativeTokens, 0)
}

func (d *DelegationOutput) FeatureSet() FeatureSet {
	return make(FeatureSet, 0)
}

func (d *DelegationOutput) UnlockConditionSet() UnlockConditionSet {
	return d.Conditions.MustSet()
}

func (d *DelegationOutput) ImmutableFeatureSet() FeatureSet {
	return d.ImmutableFeatures.MustSet()
}

func (d *DelegationOutput) Deposit() BaseToken {
	return d.Amount
}

func (d *DelegationOutput) StoredMana() Mana {
	return 0
}

func (d *DelegationOutput) Type() OutputType {
	return OutputDelegation
}

func (d *DelegationOutput) Size() int {
	return util.NumByteLen(byte(OutputDelegation)) +
		BaseTokenSize +
		BaseTokenSize +
		DelegationIDLength +
		AccountIDLength +
		EpochIndexLength*2 +
		d.Conditions.Size() +
		d.ImmutableFeatures.Size()
}