package nova

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/safemath"
	"github.com/iotaledger/hive.go/ierrors"
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/vm"
)

// NewVirtualMachine returns an VirtualMachine adhering to the Nova protocol.
func NewVirtualMachine() vm.VirtualMachine {
	return &virtualMachine{
		execList: []vm.ExecFunc{
			vm.ExecFuncTimelocks(),
			vm.ExecFuncSenderUnlocked(),
			vm.ExecFuncBalancedBaseTokens(),
			vm.ExecFuncBalancedNativeTokens(),
			vm.ExecFuncChainTransitions(),
			vm.ExecFuncBalancedMana(),
			vm.ExecFuncAtMostOneImplicitAccountCreationAddress(),
		},
	}
}

type virtualMachine struct {
	execList []vm.ExecFunc
}

func NewVMParamsWorkingSet(api iotago.API, t *iotago.Transaction, resolvedInputs vm.ResolvedInputs) (*vm.WorkingSet, error) {
	var err error
	utxoInputsSet := constructInputSet(resolvedInputs.InputSet)
	workingSet := &vm.WorkingSet{}
	workingSet.Tx = t
	workingSet.UnlockedIdents = make(vm.UnlockedIdentities)
	workingSet.UTXOInputsSet = utxoInputsSet
	workingSet.InputIDToIndex = make(map[iotago.OutputID]uint16)
	for inputIndex, inputRef := range workingSet.Tx.TransactionEssence.Inputs {
		//nolint:forcetypeassert // we can safely assume that this is an UTXOInput
		ref := inputRef.(*iotago.UTXOInput).OutputID()
		workingSet.InputIDToIndex[ref] = uint16(inputIndex)
		input, ok := workingSet.UTXOInputsSet[ref]
		if !ok {
			return nil, ierrors.Wrapf(iotago.ErrMissingUTXO, "utxo for input %d not supplied", inputIndex)
		}
		workingSet.UTXOInputs = append(workingSet.UTXOInputs, input)
	}

	workingSet.EssenceMsgToSign, err = t.SigningMessage()
	if err != nil {
		return nil, err
	}

	txID, err := workingSet.Tx.ID()
	if err != nil {
		return nil, err
	}

	workingSet.InChains = utxoInputsSet.ChainInputSet()
	workingSet.OutChains = workingSet.Tx.Outputs.ChainOutputSet(txID)

	workingSet.BIC = resolvedInputs.BlockIssuanceCreditInputSet
	workingSet.Commitment = resolvedInputs.CommitmentInput
	workingSet.Rewards = resolvedInputs.RewardsInputSet

	workingSet.TotalManaIn, err = vm.TotalManaIn(
		api.ManaDecayProvider(),
		api.StorageScoreStructure(),
		workingSet.Tx.CreationSlot,
		workingSet.UTXOInputsSet,
		workingSet.Rewards,
	)
	if err != nil {
		return nil, ierrors.Join(iotago.ErrManaAmountInvalid, err)
	}

	workingSet.TotalManaOut, err = vm.TotalManaOut(
		workingSet.Tx.Outputs,
		workingSet.Tx.Allotments,
	)
	if err != nil {
		return nil, ierrors.Join(iotago.ErrManaAmountInvalid, err)
	}

	return workingSet, nil
}

func constructInputSet(inputSet vm.InputSet) vm.InputSet {
	utxoInputsSet := vm.InputSet{}
	for outputID, outputWithCreationSlot := range inputSet {
		if basicOutput, isBasic := outputWithCreationSlot.(*iotago.BasicOutput); isBasic {
			if addressUnlock := basicOutput.UnlockConditionSet().Address(); addressUnlock != nil {
				if addressUnlock.Address.Type() == iotago.AddressImplicitAccountCreation {
					utxoInputsSet[outputID] = &vm.ImplicitAccountOutput{BasicOutput: basicOutput}

					continue
				}
			}
		}
		utxoInputsSet[outputID] = outputWithCreationSlot
	}

	return utxoInputsSet
}

func (novaVM *virtualMachine) ValidateUnlocks(signedTransaction *iotago.SignedTransaction, inputs vm.ResolvedInputs) (unlockedIdentities vm.UnlockedIdentities, err error) {
	return vm.ValidateUnlocks(signedTransaction, inputs)
}

func (novaVM *virtualMachine) Execute(transaction *iotago.Transaction, resolvedInputs vm.ResolvedInputs, unlockedIdentities vm.UnlockedIdentities, execFunctions ...vm.ExecFunc) (outputs []iotago.Output, err error) {
	vmParams := &vm.Params{
		API: transaction.API,
	}

	if vmParams.WorkingSet, err = NewVMParamsWorkingSet(vmParams.API, transaction, resolvedInputs); err != nil {
		return nil, ierrors.Wrap(err, "failed to create working set")
	}
	vmParams.WorkingSet.UnlockedIdents = unlockedIdentities

	if len(execFunctions) == 0 {
		execFunctions = novaVM.execList
	}

	err = vm.RunVMFuncs(novaVM, vmParams, execFunctions...)
	if err != nil {
		return nil, ierrors.Wrap(err, "failed to execute transaction")
	}

	outputs = make([]iotago.Output, len(transaction.Outputs))
	for i, output := range transaction.Outputs {
		outputs[i] = output
	}

	return outputs, nil
}

func (novaVM *virtualMachine) ChainSTVF(vmParams *vm.Params, transType iotago.ChainTransitionType, input *vm.ChainOutputWithIDs, next iotago.ChainOutput) error {
	transitionState := next
	if transType != iotago.ChainTransitionTypeGenesis {
		transitionState = input.Output
	}

	var ok bool
	switch castedInput := transitionState.(type) {
	case *iotago.AccountOutput:
		var nextAccount *iotago.AccountOutput
		if next != nil {
			if nextAccount, ok = next.(*iotago.AccountOutput); !ok {
				return ierrors.New("can only state transition to another account output")
			}
		}

		return accountSTVF(vmParams, input, transType, nextAccount)

	case *vm.ImplicitAccountOutput:
		var nextAccount *iotago.AccountOutput
		if next != nil {
			if nextAccount, ok = next.(*iotago.AccountOutput); !ok {
				return ierrors.New("can only state transition implicit account to an account output")
			}
		}

		return implicitAccountSTVF(vmParams, castedInput, input.OutputID, nextAccount, transType)

	case *iotago.AnchorOutput:
		var nextAnchor *iotago.AnchorOutput
		if next != nil {
			if nextAnchor, ok = next.(*iotago.AnchorOutput); !ok {
				return ierrors.New("can only state transition to another anchor output")
			}
		}

		return anchorSTVF(vmParams, input, transType, nextAnchor)

	case *iotago.FoundryOutput:
		var nextFoundry *iotago.FoundryOutput
		if next != nil {
			if nextFoundry, ok = next.(*iotago.FoundryOutput); !ok {
				return ierrors.New("can only state transition to another foundry output")
			}
		}

		return foundrySTVF(vmParams, input, transType, nextFoundry)

	case *iotago.NFTOutput:
		var nextNFT *iotago.NFTOutput
		if next != nil {
			if nextNFT, ok = next.(*iotago.NFTOutput); !ok {
				return ierrors.New("can only state transition to another NFT output")
			}
		}

		return nftSTVF(vmParams, input, transType, nextNFT)

	case *iotago.DelegationOutput:
		var nextDelegationOutput *iotago.DelegationOutput
		if next != nil {
			if nextDelegationOutput, ok = next.(*iotago.DelegationOutput); !ok {
				return ierrors.New("can only state transition to another Delegation output")
			}
		}

		return delegationSTVF(vmParams, input, transType, nextDelegationOutput)

	default:
		panic(fmt.Sprintf("invalid output type %v passed to Nova virtual machine", input.Output))
	}
}

// For implicit account conversion, there must be a basic output as input, and an account output as output with an AccountID matching the input.
func implicitAccountSTVF(vmParams *vm.Params, implicitAccount *vm.ImplicitAccountOutput, outputID iotago.OutputID, next *iotago.AccountOutput, transType iotago.ChainTransitionType) error {
	if transType == iotago.ChainTransitionTypeDestroy {
		return iotago.ErrImplicitAccountDestructionDisallowed
	}

	// Create a wrapper around the implicit account.
	implicitAccountChainOutput := &vm.ChainOutputWithIDs{
		ChainID:  next.AccountID,
		OutputID: outputID,
		Output:   implicitAccount,
	}

	implicitAccountBlockIssuerFeature := &iotago.BlockIssuerFeature{
		BlockIssuerKeys: iotago.NewBlockIssuerKeys(),
		// Setting MaxSlotIndex means one cannot remove the block issuer feature in the transition, but it does allow for setting
		// the expiry slot to a lower value, which is the behavior we want.
		ExpirySlot: iotago.MaxSlotIndex,
	}

	if err := accountBlockIssuerSTVF(vmParams, implicitAccountChainOutput, implicitAccountBlockIssuerFeature, next); err != nil {
		return err
	}

	return accountGenesisValid(vmParams, next, false)
}

// For output AccountOutput(s) with non-zeroed AccountID, there must be a corresponding input AccountOutput where either its
// AccountID is zeroed and FoundryCounter is zero or an input AccountOutput with the same AccountID.
func accountSTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, transType iotago.ChainTransitionType, next *iotago.AccountOutput) error {
	// Whether the transaction is claiming Mana rewards for this account.
	isClaimingRewards := false
	if vmParams.WorkingSet.Rewards != nil && input != nil {
		_, isClaimingRewards = vmParams.WorkingSet.Rewards[input.ChainID]
	}

	// Whether the account is removing the staking feature.
	isRemovingStakingFeatureValue := false
	isRemovingStakingFeature := &isRemovingStakingFeatureValue

	switch transType {
	case iotago.ChainTransitionTypeGenesis:
		if err := accountGenesisValid(vmParams, next, true); err != nil {
			return ierrors.Wrapf(err, " account %s", next.AccountID)
		}
	case iotago.ChainTransitionTypeStateChange:
		if err := accountStateChangeValid(vmParams, input, next, isRemovingStakingFeature); err != nil {
			//nolint:forcetypeassert // we can safely assume that this is an AccountOutput
			a := input.Output.(*iotago.AccountOutput)

			return ierrors.Wrapf(err, "account %s", a.AccountID)
		}
	case iotago.ChainTransitionTypeDestroy:
		if err := accountDestructionValid(vmParams, input, isRemovingStakingFeature); err != nil {
			//nolint:forcetypeassert // we can safely assume that this is an AccountOutput
			a := input.Output.(*iotago.AccountOutput)

			return ierrors.Wrapf(err, "account %s", a.AccountID)
		}
	default:
		panic("unknown chain transition type in AccountOutput")
	}

	if isClaimingRewards && !*isRemovingStakingFeature {
		return ierrors.Join(iotago.ErrInvalidStakingTransition, iotago.ErrInvalidStakingRewardClaim)
	}

	if !isClaimingRewards && *isRemovingStakingFeature {
		return ierrors.Join(iotago.ErrInvalidStakingTransition, iotago.ErrInvalidStakingRewardInputRequired)
	}

	return nil
}

func accountGenesisValid(vmParams *vm.Params, next *iotago.AccountOutput, accountIDMustBeZeroed bool) error {
	if accountIDMustBeZeroed && !next.AccountID.Empty() {
		return ierrors.Wrap(iotago.ErrInvalidAccountStateTransition, "AccountOutput's ID is not zeroed even though it is new")
	}

	if nextBlockIssuerFeat := next.FeatureSet().BlockIssuer(); nextBlockIssuerFeat != nil {
		if vmParams.WorkingSet.Commitment == nil {
			return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature validation requires a commitment input")
		}

		if nextBlockIssuerFeat.ExpirySlot < vmParams.PastBoundedSlotIndex(vmParams.WorkingSet.Commitment.Slot) {
			return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature expiry set too soon")
		}
	}

	if stakingFeat := next.FeatureSet().Staking(); stakingFeat != nil {
		if err := accountStakingGenesisValidation(vmParams, next, stakingFeat); err != nil {
			return ierrors.Join(iotago.ErrInvalidStakingTransition, err)
		}
	}

	return vm.IsIssuerOnOutputUnlocked(next, vmParams.WorkingSet.UnlockedIdents)
}

func accountStateChangeValid(vmParams *vm.Params, input *vm.ChainOutputWithIDs, next *iotago.AccountOutput, isRemovingStakingFeature *bool) error {
	//nolint:forcetypeassert // we can safely assume that this is an AccountOutput
	current := input.Output.(*iotago.AccountOutput)
	if !current.ImmutableFeatures.Equal(next.ImmutableFeatures) {
		return ierrors.Wrapf(iotago.ErrInvalidAccountStateTransition, "old state %s, next state %s", current.ImmutableFeatures, next.ImmutableFeatures)
	}

	// If a Block Issuer Feature is present on the input side of the transaction,
	// and the BIC is negative, the account is locked.
	if current.FeatureSet().BlockIssuer() != nil {
		if err := accountBlockIssuanceCreditLocked(input, vmParams.WorkingSet.BIC); err != nil {
			return err
		}
	}

	if err := accountStakingSTVF(vmParams, current, next, isRemovingStakingFeature); err != nil {
		return err
	}

	if err := accountFoundryCounterSTVF(vmParams, current, next); err != nil {
		return err
	}

	return accountBlockIssuerSTVF(vmParams, input, input.Output.FeatureSet().BlockIssuer(), next)
}

// If an account output has a block issuer feature, the following conditions for its transition must be checked.
// The block issuer credit must be non-negative.
// The expiry time of the block issuer feature, if creating new account or expired already, must be set at least MaxCommittableAge greater than the Commitment Input.
// Check that at least one Block Issuer Key is present.
func accountBlockIssuerSTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, currentBlockIssuerFeat *iotago.BlockIssuerFeature, next *iotago.AccountOutput) error {
	current := input.Output
	nextBlockIssuerFeat := next.FeatureSet().BlockIssuer()

	// if the account has no block issuer feature.
	if currentBlockIssuerFeat == nil && nextBlockIssuerFeat == nil {
		return nil
	}

	if vmParams.WorkingSet.Commitment == nil {
		return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature validation requires a commitment input")
	}

	// else if the account has negative bic, this is invalid.
	// new block issuers may not have a bic registered yet.
	if bic, exists := vmParams.WorkingSet.BIC[next.AccountID]; exists {
		if bic < 0 {
			return ierrors.Wrapf(iotago.ErrInvalidBlockIssuerTransition, "negative block issuer credit: %d", bic)
		}
	} else {
		return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "no BIC provided for block issuer feature validation")
	}

	commitmentInputSlot := vmParams.WorkingSet.Commitment.Slot
	pastBoundedSlot := vmParams.PastBoundedSlotIndex(commitmentInputSlot)

	if currentBlockIssuerFeat != nil && currentBlockIssuerFeat.ExpirySlot >= commitmentInputSlot {
		// if the block issuer feature has not expired, it can not be removed.
		if nextBlockIssuerFeat == nil {
			return ierrors.Wrapf(iotago.ErrInvalidBlockIssuerTransition, "cannot remove block issuer feature until it expires (current slot: %d, expiry slot: %d)", commitmentInputSlot, currentBlockIssuerFeat.ExpirySlot)
		}
		if nextBlockIssuerFeat.ExpirySlot != currentBlockIssuerFeat.ExpirySlot && nextBlockIssuerFeat.ExpirySlot < pastBoundedSlot {
			return ierrors.Wrapf(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature expiry set too soon (is %d, must be >= %d)", nextBlockIssuerFeat.ExpirySlot, pastBoundedSlot)
		}
	} else if nextBlockIssuerFeat != nil {
		// The block issuer feature was newly added,
		// or the current feature has expired but it was not removed.
		// In both cases the expiry slot must be set sufficiently far in the future.
		if nextBlockIssuerFeat.ExpirySlot < pastBoundedSlot {
			return ierrors.Wrapf(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature expiry set too soon (is %d, must be >= %d)", nextBlockIssuerFeat.ExpirySlot, pastBoundedSlot)
		}
	}

	// the Mana on the account on the input side must not be moved to any other outputs or accounts.
	manaDecayProvider := vmParams.API.ManaDecayProvider()
	storageScoreStructure := vmParams.API.StorageScoreStructure()

	manaIn := vmParams.WorkingSet.TotalManaIn
	manaOut := vmParams.WorkingSet.TotalManaOut

	// AccountInStored
	manaStoredAccount, err := manaDecayProvider.DecayManaBySlots(current.StoredMana(), input.OutputID.CreationSlot(), vmParams.WorkingSet.Tx.CreationSlot)
	if err != nil {
		return ierrors.Wrapf(err, "account %s stored mana calculation failed", next.AccountID)
	}
	manaIn, err = safemath.SafeSub(manaIn, manaStoredAccount)
	if err != nil {
		return ierrors.Wrapf(err, "account %s stored mana in exceeds total remaining mana in, manaStoredAccountIn: %d, manaIn: %d", next.AccountID, manaStoredAccount, manaIn)
	}

	// AccountInPotential - the potential mana from the input side of the account in question
	manaPotentialAccount, err := iotago.PotentialMana(manaDecayProvider, storageScoreStructure, input.Output, input.OutputID.CreationSlot(), vmParams.WorkingSet.Tx.CreationSlot)

	if err != nil {
		return ierrors.Wrapf(err, "account %s potential mana calculation failed", next.AccountID)
	}

	manaIn, err = safemath.SafeSub(manaIn, manaPotentialAccount)
	if err != nil {
		return ierrors.Wrapf(err, "account %s potential mana in exceeds total remaining mana in, manaPotentialAccountIn: %d, manaIn: %d", next.AccountID, manaPotentialAccount, manaIn)
	}

	// AccountOutStored - stored Mana on the output side of the account in question
	manaOut, err = safemath.SafeSub(manaOut, next.Mana)
	if err != nil {
		return ierrors.Wrapf(err, "account %s stored mana out exceeds total remaining mana out, storedManaOut: %d, manaOut: %d", next.AccountID, next.Mana, manaOut)
	}

	// AccountOutAllotted - allotments to the account in question
	accountOutAllotted := vmParams.WorkingSet.Tx.Allotments.Get(next.AccountID)
	manaOut, err = safemath.SafeSub(manaOut, accountOutAllotted)
	if err != nil {
		return ierrors.Wrapf(err, "account %s allotment exceeds total remaining mana out, accountAllotted: %d, manaOut: %d", next.AccountID, accountOutAllotted, manaOut)
	}

	// AccountOutLocked - outputs with manalock conditions
	minManalockedSlot := pastBoundedSlot + vmParams.API.ProtocolParameters().MaxCommittableAge()
	for outputIndex, output := range vmParams.WorkingSet.Tx.Outputs {
		if output.UnlockConditionSet().HasManalockCondition(next.AccountID, minManalockedSlot) {
			manaOut, err = safemath.SafeSub(manaOut, output.StoredMana())
			if err != nil {
				return ierrors.Wrapf(err, "account %s manalocked output mana exceeds total remaining mana out, outputIndex: %d, outputStoredMana: %d, manaOut: %d", next.AccountID, outputIndex, output.StoredMana(), manaOut)
			}
		}
	}

	if manaIn < manaOut {
		return ierrors.Wrapf(iotago.ErrInvalidBlockIssuerTransition, "cannot move Mana off an account: mana in %d, mana out %d", manaIn, manaOut)
	}

	return nil
}

func accountStakingSTVF(vmParams *vm.Params, current *iotago.AccountOutput, next *iotago.AccountOutput, isRemovingStakingFeature *bool) error {
	currentStakingFeat := current.FeatureSet().Staking()
	nextStakingFeat := next.FeatureSet().Staking()

	if currentStakingFeat != nil {

		commitment := vmParams.WorkingSet.Commitment
		if commitment == nil {
			return ierrors.Join(iotago.ErrInvalidStakingTransition, iotago.ErrInvalidStakingCommitmentInput)
		}

		timeProvider := vmParams.API.TimeProvider()
		pastBoundedSlot := vmParams.PastBoundedSlotIndex(commitment.Slot)
		pastBoundedEpoch := timeProvider.EpochFromSlot(pastBoundedSlot)
		futureBoundedSlot := vmParams.FutureBoundedSlotIndex(commitment.Slot)
		futureBoundedEpoch := timeProvider.EpochFromSlot(futureBoundedSlot)

		if futureBoundedEpoch <= currentStakingFeat.EndEpoch {
			earliestUnbondingEpoch := pastBoundedEpoch + vmParams.API.ProtocolParameters().StakingUnbondingPeriod()
			nextHasBlockIssuerFeat := next.FeatureSet().BlockIssuer() != nil

			return accountStakingNonExpiredValidation(
				currentStakingFeat, nextStakingFeat, earliestUnbondingEpoch, nextHasBlockIssuerFeat,
			)
		}

		return accountStakingExpiredValidation(vmParams, next, currentStakingFeat, nextStakingFeat, isRemovingStakingFeature)
	} else if nextStakingFeat != nil {
		return accountStakingGenesisValidation(vmParams, next, nextStakingFeat)
	}

	return nil
}

// Validates the rules for a newly added Staking Feature in an account,
// or one which was effectively removed and added within the same transaction.
// This is allowed as long as the epoch range of the old and new feature are disjoint.
func accountStakingGenesisValidation(vmParams *vm.Params, next *iotago.AccountOutput, stakingFeat *iotago.StakingFeature) error {
	// It should already never be nil here, but for 100% safety, we'll check again.
	commitment := vmParams.WorkingSet.Commitment
	if commitment == nil {
		return iotago.ErrInvalidStakingCommitmentInput
	}

	pastBoundedSlot := vmParams.PastBoundedSlotIndex(commitment.Slot)
	timeProvider := vmParams.API.TimeProvider()
	pastBoundedEpoch := timeProvider.EpochFromSlot(pastBoundedSlot)

	if stakingFeat.StartEpoch != pastBoundedEpoch {
		return iotago.ErrInvalidStakingStartEpoch
	}

	unbondingEpoch := pastBoundedEpoch + vmParams.API.ProtocolParameters().StakingUnbondingPeriod()
	if stakingFeat.EndEpoch < unbondingEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidStakingEndEpochTooEarly, "(i.e. end epoch %d should be >= %d)", stakingFeat.EndEpoch, unbondingEpoch)
	}

	if next.FeatureSet().BlockIssuer() == nil {
		return iotago.ErrInvalidStakingBlockIssuerRequired
	}

	return nil
}

// Validates a staking feature's transition if the feature is not expired,
// i.e. the current epoch is before the end epoch.
func accountStakingNonExpiredValidation(
	currentStakingFeat *iotago.StakingFeature,
	nextStakingFeat *iotago.StakingFeature,
	earliestUnbondingEpoch iotago.EpochIndex,
	nextHasBlockIssuerFeat bool,
) error {
	if nextStakingFeat == nil {
		return ierrors.Wrapf(iotago.ErrInvalidStakingTransition, "%w", iotago.ErrInvalidStakingBondedRemoval)
	}

	if !nextHasBlockIssuerFeat {
		return ierrors.Wrapf(iotago.ErrInvalidStakingTransition, "%w", iotago.ErrInvalidStakingBlockIssuerRequired)
	}

	if currentStakingFeat.StakedAmount != nextStakingFeat.StakedAmount ||
		currentStakingFeat.FixedCost != nextStakingFeat.FixedCost ||
		currentStakingFeat.StartEpoch != nextStakingFeat.StartEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidStakingTransition, "%w", iotago.ErrInvalidStakingBondedModified)
	}

	if currentStakingFeat.EndEpoch != nextStakingFeat.EndEpoch &&
		nextStakingFeat.EndEpoch < earliestUnbondingEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidStakingTransition, "%w (i.e. end epoch %d should be >= %d) or the end epoch must match on input and output side", iotago.ErrInvalidStakingEndEpochTooEarly, nextStakingFeat.EndEpoch, earliestUnbondingEpoch)
	}

	return nil
}

// Validates a staking feature's transition if the feature is expired,
// i.e. the current epoch is equal or after the end epoch.
func accountStakingExpiredValidation(
	vmParams *vm.Params,
	next *iotago.AccountOutput,
	currentStakingFeat *iotago.StakingFeature,
	nextStakingFeat *iotago.StakingFeature,
	isRemovingStakingFeature *bool,
) error {
	if nextStakingFeat == nil {
		*isRemovingStakingFeature = true
	} else if !currentStakingFeat.Equal(nextStakingFeat) {
		// If an expired feature is changed it must be transitioned as if newly added.
		if err := accountStakingGenesisValidation(vmParams, next, nextStakingFeat); err != nil {
			return ierrors.Wrapf(iotago.ErrInvalidStakingTransition, "%w: rewards claiming without removing the feature requires updating the feature", err)
		}
		// If staking feature genesis validation succeeds, the start epoch has been reset which means the new epoch range
		// is disjoint from the current staking feature's, which can therefore be considered as removing and re-adding
		// the feature.
		*isRemovingStakingFeature = true
	}

	return nil
}

func accountFoundryCounterSTVF(vmParams *vm.Params, current *iotago.AccountOutput, next *iotago.AccountOutput) error {
	if current.FoundryCounter > next.FoundryCounter {
		return ierrors.Wrapf(iotago.ErrInvalidAccountStateTransition, "foundry counter of next state is less than previous, in %d / out %d", current.FoundryCounter, next.FoundryCounter)
	}

	// check that for a foundry counter change, X amount of foundries were actually created
	if current.FoundryCounter == next.FoundryCounter {
		return nil
	}

	var seenNewFoundriesOfAccount uint32
	for _, output := range vmParams.WorkingSet.Tx.Outputs {
		foundryOutput, is := output.(*iotago.FoundryOutput)
		if !is {
			continue
		}

		if _, notNew := vmParams.WorkingSet.InChains[foundryOutput.MustFoundryID()]; notNew {
			continue
		}

		//nolint:forcetypeassert // we can safely assume that this is an AccountAddress
		foundryAccountID := foundryOutput.Ident().(*iotago.AccountAddress).ChainID()
		if !foundryAccountID.Matches(next.AccountID) {
			continue
		}
		seenNewFoundriesOfAccount++
	}

	expectedNewFoundriesCount := next.FoundryCounter - current.FoundryCounter
	if expectedNewFoundriesCount != seenNewFoundriesOfAccount {
		return ierrors.Wrapf(iotago.ErrInvalidAccountStateTransition, "%d new foundries were created but the account output's foundry counter changed by %d", seenNewFoundriesOfAccount, expectedNewFoundriesCount)
	}

	return nil
}

func accountDestructionValid(vmParams *vm.Params, input *vm.ChainOutputWithIDs, isRemovingStakingFeature *bool) error {
	if vmParams.WorkingSet.Tx.Capabilities.CannotDestroyAccountOutputs() {
		return ierrors.Join(iotago.ErrInvalidAccountStateTransition, iotago.ErrTxCapabilitiesAccountDestructionNotAllowed)
	}

	//nolint:forcetypeassert // we can safely assume that this is an AccountOutput
	outputToDestroy := input.Output.(*iotago.AccountOutput)

	blockIssuerFeat := outputToDestroy.FeatureSet().BlockIssuer()
	if blockIssuerFeat != nil {
		if vmParams.WorkingSet.Commitment == nil {
			return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "block issuer feature validation requires a commitment input")
		}

		if blockIssuerFeat.ExpirySlot >= vmParams.WorkingSet.Commitment.Slot {
			return ierrors.Wrap(iotago.ErrInvalidBlockIssuerTransition, "cannot destroy output until the block issuer feature expires")
		}

		if err := accountBlockIssuanceCreditLocked(input, vmParams.WorkingSet.BIC); err != nil {
			return err
		}
	}

	stakingFeat := outputToDestroy.FeatureSet().Staking()
	if stakingFeat != nil {
		// This case should never occur as the staking feature requires the presence of a block issuer feature,
		// which also requires a commitment input.
		commitment := vmParams.WorkingSet.Commitment
		if commitment == nil {
			return ierrors.Join(iotago.ErrInvalidStakingTransition, iotago.ErrInvalidStakingCommitmentInput)
		}

		timeProvider := vmParams.API.TimeProvider()
		futureBoundedSlot := vmParams.FutureBoundedSlotIndex(commitment.Slot)
		futureBoundedEpoch := timeProvider.EpochFromSlot(futureBoundedSlot)

		if futureBoundedEpoch <= stakingFeat.EndEpoch {
			return ierrors.Wrapf(iotago.ErrInvalidAccountStateTransition, "%w: cannot destroy account until the staking feature is unbonded", iotago.ErrInvalidStakingBondedRemoval)
		}

		*isRemovingStakingFeature = true
	}

	return nil
}

func accountBlockIssuanceCreditLocked(input *vm.ChainOutputWithIDs, bicSet vm.BlockIssuanceCreditInputSet) error {
	accountID, is := input.ChainID.(iotago.AccountID)
	if !is {
		return ierrors.Wrapf(iotago.ErrBlockIssuanceCreditInputRequired, "cannot convert chain ID %s to account ID",
			input.ChainID.ToHex())
	}

	if bic, exists := bicSet[accountID]; !exists {
		return iotago.ErrBlockIssuanceCreditInputRequired
	} else if bic < 0 {
		return ierrors.Wrapf(iotago.ErrAccountLocked, "cannot destroy locked account with ID %s", accountID.ToHex())
	}

	return nil
}

// For output AnchorOutput(s) with non-zeroed AnchorID, there must be a corresponding input AnchorOutput where either its
// AnchorID is zeroed and StateIndex is zero or an input AnchorOutput with the same AnchorID.
//
// On anchor state transitions: The StateIndex must be incremented by 1 and Only Amount, StateIndex and StateMetadata can be mutated.
//
// On anchor governance transition: Only StateController, GovernanceController and the MetadataBlock can be mutated.
func anchorSTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, transType iotago.ChainTransitionType, next *iotago.AnchorOutput) error {
	switch transType {
	case iotago.ChainTransitionTypeGenesis:
		if err := anchorGenesisValid(vmParams, next, true); err != nil {
			return ierrors.Wrapf(err, " anchor %s", next.AnchorID)
		}
	case iotago.ChainTransitionTypeStateChange:
		if err := anchorStateChangeValid(input, next); err != nil {
			//nolint:forcetypeassert // we can safely assume that this is an AnchorOutput
			a := input.Output.(*iotago.AnchorOutput)

			return ierrors.Wrapf(err, "anchor %s", a.AnchorID)
		}
	case iotago.ChainTransitionTypeDestroy:
		if err := anchorDestructionValid(vmParams); err != nil {
			//nolint:forcetypeassert // we can safely assume that this is an AnchorOutput
			a := input.Output.(*iotago.AnchorOutput)

			return ierrors.Wrapf(err, "anchor %s", a.AnchorID)
		}
	default:
		panic("unknown chain transition type in AnchorOutput")
	}

	return nil
}

func anchorGenesisValid(vmParams *vm.Params, current *iotago.AnchorOutput, anchorIDMustBeZeroed bool) error {
	if anchorIDMustBeZeroed && !current.AnchorID.Empty() {
		return ierrors.Wrap(iotago.ErrInvalidAnchorStateTransition, "AnchorOutput's ID is not zeroed even though it is new")
	}

	return vm.IsIssuerOnOutputUnlocked(current, vmParams.WorkingSet.UnlockedIdents)
}

func anchorStateChangeValid(input *vm.ChainOutputWithIDs, next *iotago.AnchorOutput) error {
	//nolint:forcetypeassert // we can safely assume that this is an AnchorOutput
	current := input.Output.(*iotago.AnchorOutput)

	isGovTransition := current.StateIndex == next.StateIndex
	if !current.ImmutableFeatures.Equal(next.ImmutableFeatures) {
		err := iotago.ErrInvalidAnchorStateTransition
		if isGovTransition {
			err = iotago.ErrInvalidAnchorGovernanceTransition
		}

		return ierrors.Wrapf(err, "old state %s, next state %s", current.ImmutableFeatures, next.ImmutableFeatures)
	}

	if isGovTransition {
		return anchorGovernanceSTVF(input, next)
	}

	return anchorStateSTVF(input, next)
}

func anchorGovernanceSTVF(input *vm.ChainOutputWithIDs, next *iotago.AnchorOutput) error {
	//nolint:forcetypeassert // we can safely assume that this is an AnchorOutput
	current := input.Output.(*iotago.AnchorOutput)

	switch {
	case current.Amount != next.Amount:
		return ierrors.Wrapf(iotago.ErrInvalidAnchorGovernanceTransition, "amount changed, in %d / out %d ", current.Amount, next.Amount)
	case current.StateIndex != next.StateIndex:
		return ierrors.Wrapf(iotago.ErrInvalidAnchorGovernanceTransition, "state index changed, in %d / out %d", current.StateIndex, next.StateIndex)
	}

	if err := iotago.FeatureUnchanged(iotago.FeatureStateMetadata, current.Features.MustSet(), next.Features.MustSet()); err != nil {
		return ierrors.Wrapf(iotago.ErrInvalidAnchorGovernanceTransition, "%w", err)
	}

	return nil
}

func anchorStateSTVF(input *vm.ChainOutputWithIDs, next *iotago.AnchorOutput) error {
	//nolint:forcetypeassert // we can safely assume that this is an AnchorOutput
	current := input.Output.(*iotago.AnchorOutput)
	switch {
	case !current.StateController().Equal(next.StateController()):
		return ierrors.Wrapf(iotago.ErrInvalidAnchorStateTransition, "state controller changed, in %v / out %v", current.StateController(), next.StateController())
	case !current.GovernorAddress().Equal(next.GovernorAddress()):
		return ierrors.Wrapf(iotago.ErrInvalidAnchorStateTransition, "governance controller changed, in %v / out %v", current.GovernorAddress(), next.GovernorAddress())
	case current.StateIndex+1 != next.StateIndex:
		return ierrors.Wrapf(iotago.ErrInvalidAnchorStateTransition, "state index %d on the input side but %d on the output side", current.StateIndex, next.StateIndex)
	}

	if err := iotago.FeatureUnchanged(iotago.FeatureMetadata, current.Features.MustSet(), next.Features.MustSet()); err != nil {
		return ierrors.Wrapf(iotago.ErrInvalidAnchorStateTransition, "%w", err)
	}

	return nil
}

func anchorDestructionValid(vmParams *vm.Params) error {
	if vmParams.WorkingSet.Tx.Capabilities.CannotDestroyAnchorOutputs() {
		return ierrors.Join(iotago.ErrInvalidAnchorStateTransition, iotago.ErrTxCapabilitiesAnchorDestructionNotAllowed)
	}

	return nil
}

func foundrySTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, transType iotago.ChainTransitionType, next *iotago.FoundryOutput) error {
	inSums := vmParams.WorkingSet.InNativeTokens
	outSums := vmParams.WorkingSet.OutNativeTokens

	switch transType {
	case iotago.ChainTransitionTypeGenesis:
		if err := foundryGenesisValid(vmParams, next, next.MustFoundryID(), outSums); err != nil {
			return ierrors.Wrapf(err, "foundry %s, token %s", next.MustFoundryID(), next.MustNativeTokenID())
		}
	case iotago.ChainTransitionTypeStateChange:
		//nolint:forcetypeassert // we can safely assume that this is a FoundryOutput
		current := input.Output.(*iotago.FoundryOutput)
		if err := foundryStateChangeValid(current, next, inSums, outSums); err != nil {
			return ierrors.Wrapf(err, "foundry %s, token %s", current.MustFoundryID(), current.MustNativeTokenID())
		}
	case iotago.ChainTransitionTypeDestroy:
		//nolint:forcetypeassert // we can safely assume that this is a FoundryOutput
		current := input.Output.(*iotago.FoundryOutput)
		if err := foundryDestructionValid(vmParams, current, inSums, outSums); err != nil {
			return ierrors.Wrapf(err, "foundry %s, token %s", current.MustFoundryID(), current.MustNativeTokenID())
		}
	default:
		panic("unknown chain transition type in FoundryOutput")
	}

	return nil
}

func foundryGenesisValid(vmParams *vm.Params, current *iotago.FoundryOutput, thisFoundryID iotago.FoundryID, outSums iotago.NativeTokenSum) error {
	nativeTokenID := current.MustNativeTokenID()
	if err := current.TokenScheme.StateTransition(iotago.ChainTransitionTypeGenesis, nil, nil, outSums.ValueOrBigInt0(nativeTokenID)); err != nil {
		return err
	}

	// grab foundry counter from transitioning AccountOutput
	//nolint:forcetypeassert // we can safely assume that this is an AccountAddress
	accountID := current.Ident().(*iotago.AccountAddress).AccountID()
	inAccount, ok := vmParams.WorkingSet.InChains[accountID]
	if !ok {
		return ierrors.Wrapf(iotago.ErrInvalidFoundryStateTransition, "missing input transitioning account output %s for new foundry output %s", accountID, thisFoundryID)
	}

	outAccount, ok := vmParams.WorkingSet.OutChains[accountID]
	if !ok {
		return ierrors.Wrapf(iotago.ErrInvalidFoundryStateTransition, "missing output transitioning account output %s for new foundry output %s", accountID, thisFoundryID)
	}

	//nolint:forcetypeassert // we can safely assume that this is an AccountOutput
	return foundrySerialNumberValid(vmParams, current, inAccount.Output.(*iotago.AccountOutput), outAccount.(*iotago.AccountOutput), thisFoundryID)
}

func foundrySerialNumberValid(vmParams *vm.Params, current *iotago.FoundryOutput, inAccount *iotago.AccountOutput, outAccount *iotago.AccountOutput, thisFoundryID iotago.FoundryID) error {
	// this new foundry's serial number must be between the given foundry counter interval
	startSerial := inAccount.FoundryCounter
	endIncSerial := outAccount.FoundryCounter
	if startSerial >= current.SerialNumber || current.SerialNumber > endIncSerial {
		return ierrors.Wrapf(iotago.ErrInvalidFoundryStateTransition, "new foundry output %s's serial number is not between the foundry counter interval of [%d,%d)", thisFoundryID, startSerial, endIncSerial)
	}

	// OPTIMIZE: this loop happens on every STVF of every new foundry output
	// check order of serial number
	for outputIndex, output := range vmParams.WorkingSet.Tx.Outputs {
		otherFoundryOutput, is := output.(*iotago.FoundryOutput)
		if !is {
			continue
		}

		if !otherFoundryOutput.Ident().Equal(current.Ident()) {
			continue
		}

		otherFoundryID, err := otherFoundryOutput.FoundryID()
		if err != nil {
			return err
		}

		if _, isNotNew := vmParams.WorkingSet.InChains[otherFoundryID]; isNotNew {
			continue
		}

		// only check up to own foundry whether it is ordered
		if otherFoundryID == thisFoundryID {
			break
		}

		if otherFoundryOutput.SerialNumber >= current.SerialNumber {
			return ierrors.Wrapf(iotago.ErrInvalidFoundryStateTransition, "new foundry output %s at index %d has bigger equal serial number than this foundry %s", otherFoundryID, outputIndex, thisFoundryID)
		}
	}

	return nil
}

func foundryStateChangeValid(current *iotago.FoundryOutput, next *iotago.FoundryOutput, inSums iotago.NativeTokenSum, outSums iotago.NativeTokenSum) error {
	if !current.ImmutableFeatures.Equal(next.ImmutableFeatures) {
		return ierrors.Wrapf(iotago.ErrInvalidFoundryStateTransition, "old state %s, next state %s", current.ImmutableFeatures, next.ImmutableFeatures)
	}

	// the check for the serial number and token scheme not being mutated is implicit
	// as a change would cause the foundry ID to be different, which would result in
	// no matching foundry to be found to validate the state transition against
	if current.MustFoundryID() != next.MustFoundryID() {
		// impossible invariant as the STVF should be called via the matching next foundry output
		panic(fmt.Sprintf("foundry IDs mismatch in state transition validation function: have %v got %v", current.MustFoundryID(), next.MustFoundryID()))
	}

	nativeTokenID := current.MustNativeTokenID()

	return current.TokenScheme.StateTransition(iotago.ChainTransitionTypeStateChange, next.TokenScheme, inSums.ValueOrBigInt0(nativeTokenID), outSums.ValueOrBigInt0(nativeTokenID))
}

func foundryDestructionValid(vmParams *vm.Params, current *iotago.FoundryOutput, inSums iotago.NativeTokenSum, outSums iotago.NativeTokenSum) error {
	if vmParams.WorkingSet.Tx.Capabilities.CannotDestroyFoundryOutputs() {
		return ierrors.Join(iotago.ErrInvalidFoundryStateTransition, iotago.ErrTxCapabilitiesFoundryDestructionNotAllowed)
	}

	nativeTokenID := current.MustNativeTokenID()

	return current.TokenScheme.StateTransition(iotago.ChainTransitionTypeDestroy, nil, inSums.ValueOrBigInt0(nativeTokenID), outSums.ValueOrBigInt0(nativeTokenID))
}

func nftSTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, transType iotago.ChainTransitionType, next *iotago.NFTOutput) error {
	switch transType {
	case iotago.ChainTransitionTypeGenesis:
		if err := nftGenesisValid(vmParams, next); err != nil {
			return &iotago.ChainTransitionError{Inner: err, Msg: fmt.Sprintf("NFT %s", next.NFTID)}
		}
	case iotago.ChainTransitionTypeStateChange:
		//nolint:forcetypeassert // we can safely assume that this is an NFTOutput
		current := input.Output.(*iotago.NFTOutput)
		if err := nftStateChangeValid(current, next); err != nil {
			return &iotago.ChainTransitionError{Inner: err, Msg: fmt.Sprintf("NFT %s", current.NFTID)}
		}
	case iotago.ChainTransitionTypeDestroy:
		//nolint:forcetypeassert // we can safely assume that this is an NFTOutput
		current := input.Output.(*iotago.NFTOutput)
		if err := nftDestructionValid(vmParams); err != nil {
			return &iotago.ChainTransitionError{Inner: err, Msg: fmt.Sprintf("NFT %s", current.NFTID)}
		}
	default:
		panic("unknown chain transition type in NFTOutput")
	}

	return nil
}

func nftGenesisValid(vmParams *vm.Params, current *iotago.NFTOutput) error {
	if !current.NFTID.Empty() {
		return ierrors.New("NFTOutput's ID is not zeroed even though it is new")
	}

	return vm.IsIssuerOnOutputUnlocked(current, vmParams.WorkingSet.UnlockedIdents)
}

func nftStateChangeValid(current *iotago.NFTOutput, next *iotago.NFTOutput) error {
	if !current.ImmutableFeatures.Equal(next.ImmutableFeatures) {
		return ierrors.Errorf("old state %s, next state %s", current.ImmutableFeatures, next.ImmutableFeatures)
	}

	return nil
}

func nftDestructionValid(vmParams *vm.Params) error {
	if vmParams.WorkingSet.Tx.Capabilities.CannotDestroyNFTOutputs() {
		return ierrors.Join(iotago.ErrInvalidNFTStateTransition, iotago.ErrTxCapabilitiesNFTDestructionNotAllowed)
	}

	return nil
}

func delegationSTVF(vmParams *vm.Params, input *vm.ChainOutputWithIDs, transType iotago.ChainTransitionType, next *iotago.DelegationOutput) error {
	switch transType {
	case iotago.ChainTransitionTypeGenesis:
		if err := delegationGenesisValid(vmParams, next); err != nil {
			return &iotago.ChainTransitionError{Inner: err, Msg: fmt.Sprintf("Delegation %s", next.DelegationID)}
		}
	case iotago.ChainTransitionTypeStateChange:
		_, isClaiming := vmParams.WorkingSet.Rewards[input.ChainID]
		if isClaiming {
			return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w: cannot claim rewards during delegation output transition", iotago.ErrInvalidDelegationRewardsClaiming)
		}
		//nolint:forcetypeassert // we can safely assume that this is an DelegationOutput
		current := input.Output.(*iotago.DelegationOutput)
		if err := delegationStateChangeValid(vmParams, current, next); err != nil {
			return &iotago.ChainTransitionError{Inner: err, Msg: fmt.Sprintf("Delegation %s", current.DelegationID)}
		}
	case iotago.ChainTransitionTypeDestroy:
		_, isClaiming := vmParams.WorkingSet.Rewards[input.ChainID]
		if !isClaiming {
			return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w: cannot destroy delegation output without a rewards input", iotago.ErrInvalidDelegationRewardsClaiming)
		}

		return nil
	default:
		panic("unknown chain transition type in DelegationOutput")
	}

	return nil
}

func delegationGenesisValid(vmParams *vm.Params, current *iotago.DelegationOutput) error {
	if !current.DelegationID.Empty() {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationNonZeroedID, "%w", iotago.ErrInvalidDelegationTransition)
	}

	timeProvider := vmParams.API.TimeProvider()
	commitment := vmParams.WorkingSet.Commitment
	if commitment == nil {
		return iotago.ErrDelegationCommitmentInputRequired
	}
	pastBoundedSlot := vmParams.PastBoundedSlotIndex(commitment.Slot)
	pastBoundedEpoch := timeProvider.EpochFromSlot(pastBoundedSlot)
	registrationSlot := registrationSlot(vmParams, pastBoundedEpoch)

	var expectedStartEpoch iotago.EpochIndex
	if pastBoundedSlot <= registrationSlot {
		expectedStartEpoch = pastBoundedEpoch + 1
	} else {
		expectedStartEpoch = pastBoundedEpoch + 2
	}

	if current.StartEpoch != expectedStartEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w (is %d, expected %d)", iotago.ErrInvalidDelegationStartEpoch, current.StartEpoch, expectedStartEpoch)
	}

	if current.DelegatedAmount != current.Amount {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w", iotago.ErrInvalidDelegationAmount)
	}

	if current.EndEpoch != 0 {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w", iotago.ErrInvalidDelegationNonZeroEndEpoch)
	}

	return nil
}

func delegationStateChangeValid(vmParams *vm.Params, current *iotago.DelegationOutput, next *iotago.DelegationOutput) error {
	// State transitioning a Delegation Output is always a transition to the delayed claiming state.
	// Since they can only be transitioned once, the input will always need to have a zeroed ID.
	if !current.DelegationID.Empty() {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w: delegation output can only be transitioned if it has a zeroed ID", iotago.ErrInvalidDelegationNonZeroedID)
	}

	if current.DelegatedAmount != next.DelegatedAmount ||
		!current.ValidatorAddress.Equal(next.ValidatorAddress) ||
		current.StartEpoch != next.StartEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w", iotago.ErrInvalidDelegationModified)
	}

	timeProvider := vmParams.API.TimeProvider()
	commitment := vmParams.WorkingSet.Commitment
	if commitment == nil {
		return iotago.ErrDelegationCommitmentInputRequired
	}
	futureBoundedSlot := vmParams.FutureBoundedSlotIndex(commitment.Slot)
	futureBoundedEpoch := timeProvider.EpochFromSlot(futureBoundedSlot)
	registrationSlot := registrationSlot(vmParams, futureBoundedEpoch)

	var expectedEndEpoch iotago.EpochIndex
	if futureBoundedSlot <= registrationSlot {
		expectedEndEpoch = futureBoundedEpoch
	} else {
		expectedEndEpoch = futureBoundedEpoch + 1
	}

	if next.EndEpoch != expectedEndEpoch {
		return ierrors.Wrapf(iotago.ErrInvalidDelegationTransition, "%w (is %d, expected %d)", iotago.ErrInvalidDelegationEndEpoch, next.EndEpoch, expectedEndEpoch)
	}

	return nil
}

// registrationSlot returns the slot at the end of which the validator and delegator registration ends and the voting power
// for the epoch with index epoch + 1 is calculated.
func registrationSlot(vmParams *vm.Params, epoch iotago.EpochIndex) iotago.SlotIndex {
	return vmParams.API.TimeProvider().EpochEnd(epoch) - vmParams.API.ProtocolParameters().EpochNearingThreshold()
}
