//nolint:dupl
package iotago

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/safemath"
	"github.com/iotaledger/hive.go/ierrors"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/iota.go/v4/merklehasher"
)

const (
	apiV3Version = 3
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func disallowImplicitAccountCreationAddress(address Address) error {
	if address.Type() == AddressImplicitAccountCreation {
		return ErrImplicitAccountCreationAddressInInvalidUnlockCondition
	}

	return nil
}

var (
	basicOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 1, // Min: AddressUnlockCondition
		Max: 4, // Max: AddressUnlockCondition, StorageDepositReturnUnlockCondition, TimelockUnlockCondition, ExpirationUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionAddress): struct{}{},
		},
	}
	basicOutputV3FeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 4, // Max: SenderFeature, MetadataFeature, TagFeature, NativeTokenFeature
	}

	accountOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 1, // Min: AddressUnlockCondition
		Max: 1, // Max: AddressUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionAddress): struct{}{},
		},
	}

	accountOutputV3FeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 4, // Max: SenderFeature, MetadataFeature, BlockIssuerFeature, StakingFeature
	}

	accountOutputV3ImmFeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 2, // Max: IssuerFeature, MetadataFeature
	}

	anchorOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 2, // Min: StateControllerAddressUnlockCondition, GovernorAddressUnlockCondition
		Max: 2, // Max: StateControllerAddressUnlockCondition, GovernorAddressUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionStateControllerAddress): struct{}{},
			uint32(UnlockConditionGovernorAddress):        struct{}{},
		},
	}

	anchorOutputV3FeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 2, // Max: MetadataFeature, StateMetadataFeature
	}

	anchorOutputV3ImmFeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 2, // Max: IssuerFeature, MetadataFeature
	}

	foundryOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 1, // Min: ImmutableAccountUnlockCondition
		Max: 1, // Max: ImmutableAccountUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionImmutableAccount): struct{}{},
		},
	}

	foundryOutputV3FeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 2, // Max: MetadataFeature, NativeTokenFeature
	}

	foundryOutputV3ImmFeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 1, // Max: MetadataFeature
	}

	nftOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 1, // Min: AddressUnlockCondition
		Max: 4, // Max: AddressUnlockCondition, StorageDepositReturnUnlockCondition, TimelockUnlockCondition, ExpirationUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionAddress): struct{}{},
		},
	}

	nftOutputV3FeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 3, // Max: SenderFeature, MetadataFeature, TagFeature
	}

	nftOutputV3ImmFeatBlocksArrRules = &serix.ArrayRules{
		Min: 0, // Min: -
		Max: 2, // Max: IssuerFeature, MetadataFeature
	}

	delegationOutputV3UnlockCondArrRules = &serix.ArrayRules{
		Min: 1, // Min: AddressUnlockCondition
		Max: 1, // Max: AddressUnlockCondition
		MustOccur: serializer.TypePrefixes{
			uint32(UnlockConditionAddress): struct{}{},
		},
	}

	txEssenceV3ContextInputsArrRules = &serix.ArrayRules{
		Min: MinContextInputsCount,
		Max: MaxContextInputsCount,
	}

	txEssenceV3InputsArrRules = &serix.ArrayRules{
		Min: MinInputsCount,
		Max: MaxInputsCount,
	}

	txEssenceV3OutputsArrRules = &serix.ArrayRules{
		Min: MinOutputsCount,
		Max: MaxOutputsCount,
	}

	txEssenceV3AllotmentsArrRules = &serix.ArrayRules{
		Min: MinAllotmentCount,
		Max: MaxAllotmentCount,
	}

	txV3UnlocksArrRules = &serix.ArrayRules{
		Min: 1,
		Max: MaxInputsCount,
	}
)

// v3api implements the iota-core 1.0 protocol core models.
type v3api struct {
	serixAPI *serix.API

	protocolParameters        *V3ProtocolParameters
	timeProvider              *TimeProvider
	manaDecayProvider         *ManaDecayProvider
	livenessThresholdDuration time.Duration
	storageScoreStructure     *StorageScoreStructure
	maxBlockWork              WorkScore
	computedInitialReward     Mana
	computedFinalReward       Mana
}

type contextAPIKey = struct{}

func APIFromContext(ctx context.Context) API {
	//nolint:forcetypeassert // we can safely assume that this is an API
	return ctx.Value(contextAPIKey{}).(API)
}

func (v *v3api) Equals(other API) bool {
	return v.protocolParameters.Equals(other.ProtocolParameters())
}

func (v *v3api) context() context.Context {
	return context.WithValue(context.Background(), contextAPIKey{}, v)
}

func (v *v3api) JSONEncode(obj any, opts ...serix.Option) ([]byte, error) {
	return v.serixAPI.JSONEncode(v.context(), obj, opts...)
}

func (v *v3api) JSONDecode(jsonData []byte, obj any, opts ...serix.Option) error {
	return v.serixAPI.JSONDecode(v.context(), jsonData, obj, opts...)
}

func (v *v3api) Underlying() *serix.API {
	return v.serixAPI
}

func (v *v3api) Version() Version {
	return v.protocolParameters.Version()
}

func (v *v3api) ProtocolParameters() ProtocolParameters {
	return v.protocolParameters
}

func (v *v3api) StorageScoreStructure() *StorageScoreStructure {
	return v.storageScoreStructure
}

func (v *v3api) TimeProvider() *TimeProvider {
	return v.timeProvider
}

func (v *v3api) ManaDecayProvider() *ManaDecayProvider {
	return v.manaDecayProvider
}

func (v *v3api) LivenessThresholdDuration() time.Duration {
	return v.livenessThresholdDuration
}

func (v *v3api) MaxBlockWork() WorkScore {
	return v.maxBlockWork
}

func (v *v3api) ComputedInitialReward() Mana {
	return v.computedInitialReward
}

func (v *v3api) ComputedFinalReward() Mana {
	return v.computedFinalReward
}

func (v *v3api) Encode(obj interface{}, opts ...serix.Option) ([]byte, error) {
	return v.serixAPI.Encode(v.context(), obj, opts...)
}

func (v *v3api) Decode(b []byte, obj interface{}, opts ...serix.Option) (int, error) {
	return v.serixAPI.Decode(v.context(), b, obj, opts...)
}

// V3API instantiates an API instance with types registered conforming to protocol version 3 (iota-core 1.0) of the IOTA protocol.
func V3API(protoParams ProtocolParameters) API {
	api := CommonSerixAPI()

	timeProvider := NewTimeProvider(protoParams.GenesisSlot(), protoParams.GenesisUnixTimestamp(), int64(protoParams.SlotDurationInSeconds()), protoParams.SlotsPerEpochExponent())

	maxBlockWork, err := protoParams.WorkScoreParameters().MaxBlockWork()
	must(err)

	initialReward, finalReward, err := calculateRewards(protoParams)
	must(err)

	//nolint:forcetypeassert // we can safely assume that these are V3ProtocolParameters
	v3 := &v3api{
		serixAPI:              api,
		protocolParameters:    protoParams.(*V3ProtocolParameters),
		storageScoreStructure: NewStorageScoreStructure(protoParams.StorageScoreParameters()),
		timeProvider:          timeProvider,
		manaDecayProvider:     NewManaDecayProvider(timeProvider, protoParams.SlotsPerEpochExponent(), protoParams.ManaParameters()),
		maxBlockWork:          maxBlockWork,
		computedInitialReward: initialReward,
		computedFinalReward:   finalReward,
	}

	must(api.RegisterTypeSettings(TaggedData{},
		serix.TypeSettings{}.WithObjectType(uint8(PayloadTaggedData))),
	)

	must(api.RegisterTypeSettings(CandidacyAnnouncement{},
		serix.TypeSettings{}.WithObjectType(uint8(PayloadCandidacyAnnouncement))),
	)

	{
		must(api.RegisterTypeSettings(Ed25519Signature{},
			serix.TypeSettings{}.WithObjectType(uint8(SignatureEd25519))),
		)
		must(api.RegisterInterfaceObjects((*Signature)(nil), (*Ed25519Signature)(nil)))
	}

	{
		must(api.RegisterTypeSettings(SenderFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureSender))),
		)
		must(api.RegisterTypeSettings(IssuerFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureIssuer))),
		)

		must(api.RegisterTypeSettings(MetadataFeatureEntriesKey(""),
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte)),
		)
		must(api.RegisterValidator(MetadataFeatureEntriesKey(""), func(ctx context.Context, key MetadataFeatureEntriesKey) error {
			if err := checkPrintableASCIIString(string(key)); err != nil {
				return ierrors.Join(ErrInvalidMetadataKey, err)
			}

			return nil
		}))
		must(api.RegisterTypeSettings(MetadataFeatureEntriesValue{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16)),
		)
		must(api.RegisterTypeSettings(MetadataFeatureEntries{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithMinLen(1)),
		)
		must(api.RegisterTypeSettings(MetadataFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureMetadata))),
		)

		must(api.RegisterTypeSettings(StateMetadataFeatureEntriesKey(""),
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte)),
		)
		must(api.RegisterValidator(StateMetadataFeatureEntriesKey(""), func(ctx context.Context, key StateMetadataFeatureEntriesKey) error {
			if err := checkPrintableASCIIString(string(key)); err != nil {
				return ierrors.Join(ErrInvalidStateMetadataKey, err)
			}

			return nil
		}))
		must(api.RegisterTypeSettings(StateMetadataFeatureEntriesValue{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16)),
		)
		must(api.RegisterTypeSettings(StateMetadataFeatureEntries{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithMinLen(1)),
		)
		must(api.RegisterTypeSettings(StateMetadataFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureStateMetadata))),
		)

		must(api.RegisterTypeSettings(TagFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureTag))),
		)
		must(api.RegisterTypeSettings(NativeTokenFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureNativeToken))),
		)
		must(api.RegisterTypeSettings(BlockIssuerFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureBlockIssuer))),
		)
		must(api.RegisterTypeSettings(StakingFeature{},
			serix.TypeSettings{}.WithObjectType(uint8(FeatureStaking))),
		)
	}

	{
		must(api.RegisterTypeSettings(AddressUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionAddress))),
		)
		must(api.RegisterTypeSettings(StorageDepositReturnUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionStorageDepositReturn))),
		)
		must(api.RegisterValidator(StorageDepositReturnUnlockCondition{},
			func(ctx context.Context, sdruc StorageDepositReturnUnlockCondition) error {
				return disallowImplicitAccountCreationAddress(sdruc.ReturnAddress)
			},
		))
		must(api.RegisterTypeSettings(TimelockUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionTimelock))),
		)
		must(api.RegisterTypeSettings(ExpirationUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionExpiration))),
		)
		must(api.RegisterValidator(ExpirationUnlockCondition{},
			func(ctx context.Context, exp ExpirationUnlockCondition) error {
				return disallowImplicitAccountCreationAddress(exp.ReturnAddress)
			},
		))
		must(api.RegisterTypeSettings(StateControllerAddressUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionStateControllerAddress))),
		)
		must(api.RegisterValidator(StateControllerAddressUnlockCondition{},
			func(ctx context.Context, stateController StateControllerAddressUnlockCondition) error {
				return disallowImplicitAccountCreationAddress(stateController.Address)
			},
		))
		must(api.RegisterTypeSettings(GovernorAddressUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionGovernorAddress))),
		)
		must(api.RegisterValidator(GovernorAddressUnlockCondition{},
			func(ctx context.Context, gov GovernorAddressUnlockCondition) error {
				return disallowImplicitAccountCreationAddress(gov.Address)
			},
		))
		must(api.RegisterTypeSettings(ImmutableAccountUnlockCondition{},
			serix.TypeSettings{}.WithObjectType(uint8(UnlockConditionImmutableAccount))),
		)
	}

	{
		must(api.RegisterTypeSettings(SignatureUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockSignature))))
		must(api.RegisterTypeSettings(ReferenceUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockReference))))
		must(api.RegisterTypeSettings(AccountUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockAccount))))
		must(api.RegisterTypeSettings(AnchorUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockAnchor))))
		must(api.RegisterTypeSettings(NFTUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockNFT))))
		must(api.RegisterTypeSettings(MultiUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockMulti))))
		must(api.RegisterTypeSettings(EmptyUnlock{}, serix.TypeSettings{}.WithObjectType(uint8(UnlockEmpty))))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*SignatureUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*ReferenceUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*AccountUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*AnchorUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*NFTUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*MultiUnlock)(nil)))
		must(api.RegisterInterfaceObjects((*Unlock)(nil), (*EmptyUnlock)(nil)))
	}

	{
		must(api.RegisterTypeSettings(BasicOutput{}, serix.TypeSettings{}.WithObjectType(uint8(OutputBasic))))

		must(api.RegisterTypeSettings(BasicOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(basicOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*BasicOutputUnlockCondition)(nil), (*AddressUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputUnlockCondition)(nil), (*StorageDepositReturnUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputUnlockCondition)(nil), (*TimelockUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputUnlockCondition)(nil), (*ExpirationUnlockCondition)(nil)))

		must(api.RegisterTypeSettings(BasicOutputFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(basicOutputV3FeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*BasicOutputFeature)(nil), (*SenderFeature)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputFeature)(nil), (*MetadataFeature)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputFeature)(nil), (*TagFeature)(nil)))
		must(api.RegisterInterfaceObjects((*BasicOutputFeature)(nil), (*NativeTokenFeature)(nil)))
	}

	{
		must(api.RegisterTypeSettings(AccountOutput{}, serix.TypeSettings{}.WithObjectType(uint8(OutputAccount))))

		must(api.RegisterTypeSettings(AccountOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(accountOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*AccountOutputUnlockCondition)(nil), (*AddressUnlockCondition)(nil)))

		must(api.RegisterTypeSettings(AccountOutputFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(accountOutputV3FeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*AccountOutputFeature)(nil), (*SenderFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AccountOutputFeature)(nil), (*MetadataFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AccountOutputFeature)(nil), (*BlockIssuerFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AccountOutputFeature)(nil), (*StakingFeature)(nil)))

		must(api.RegisterTypeSettings(AccountOutputImmFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(accountOutputV3ImmFeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*AccountOutputImmFeature)(nil), (*IssuerFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AccountOutputImmFeature)(nil), (*MetadataFeature)(nil)))
	}

	{
		must(api.RegisterTypeSettings(AnchorOutput{}, serix.TypeSettings{}.WithObjectType(uint8(OutputAnchor))))

		must(api.RegisterTypeSettings(AnchorOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(anchorOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*AnchorOutputUnlockCondition)(nil), (*StateControllerAddressUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*AnchorOutputUnlockCondition)(nil), (*GovernorAddressUnlockCondition)(nil)))

		must(api.RegisterTypeSettings(AnchorOutputFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(anchorOutputV3FeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*AnchorOutputFeature)(nil), (*MetadataFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AnchorOutputFeature)(nil), (*StateMetadataFeature)(nil)))

		must(api.RegisterTypeSettings(AnchorOutputImmFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(anchorOutputV3ImmFeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*AnchorOutputImmFeature)(nil), (*IssuerFeature)(nil)))
		must(api.RegisterInterfaceObjects((*AnchorOutputImmFeature)(nil), (*MetadataFeature)(nil)))
	}

	{
		must(api.RegisterTypeSettings(FoundryOutput{},
			serix.TypeSettings{}.WithObjectType(uint8(OutputFoundry))),
		)

		must(api.RegisterTypeSettings(FoundryOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(foundryOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*FoundryOutputUnlockCondition)(nil), (*ImmutableAccountUnlockCondition)(nil)))

		must(api.RegisterTypeSettings(FoundryOutputFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(foundryOutputV3FeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*FoundryOutputFeature)(nil), (*MetadataFeature)(nil)))
		must(api.RegisterInterfaceObjects((*FoundryOutputFeature)(nil), (*NativeTokenFeature)(nil)))

		must(api.RegisterTypeSettings(FoundryOutputImmFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(foundryOutputV3ImmFeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*FoundryOutputImmFeature)(nil), (*MetadataFeature)(nil)))

		must(api.RegisterTypeSettings(SimpleTokenScheme{}, serix.TypeSettings{}.WithObjectType(uint8(TokenSchemeSimple))))
		must(api.RegisterInterfaceObjects((*TokenScheme)(nil), (*SimpleTokenScheme)(nil)))
	}

	{
		must(api.RegisterTypeSettings(NFTOutput{},
			serix.TypeSettings{}.WithObjectType(uint8(OutputNFT))),
		)

		must(api.RegisterTypeSettings(NFTOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(nftOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*NFTOutputUnlockCondition)(nil), (*AddressUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputUnlockCondition)(nil), (*StorageDepositReturnUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputUnlockCondition)(nil), (*TimelockUnlockCondition)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputUnlockCondition)(nil), (*ExpirationUnlockCondition)(nil)))

		must(api.RegisterTypeSettings(NFTOutputFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(nftOutputV3FeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*NFTOutputFeature)(nil), (*SenderFeature)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputFeature)(nil), (*MetadataFeature)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputFeature)(nil), (*TagFeature)(nil)))

		must(api.RegisterTypeSettings(NFTOutputImmFeatures{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(nftOutputV3ImmFeatBlocksArrRules),
		))

		must(api.RegisterInterfaceObjects((*NFTOutputImmFeature)(nil), (*IssuerFeature)(nil)))
		must(api.RegisterInterfaceObjects((*NFTOutputImmFeature)(nil), (*MetadataFeature)(nil)))
	}

	{
		must(api.RegisterTypeSettings(DelegationOutput{}, serix.TypeSettings{}.WithObjectType(uint8(OutputDelegation))))

		must(api.RegisterTypeSettings(DelegationOutputUnlockConditions{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(delegationOutputV3UnlockCondArrRules),
		))

		must(api.RegisterInterfaceObjects((*DelegationOutputUnlockCondition)(nil), (*AddressUnlockCondition)(nil)))
	}

	{
		must(api.RegisterTypeSettings(CommitmentInput{},
			serix.TypeSettings{}.WithObjectType(uint8(ContextInputCommitment))),
		)
		must(api.RegisterTypeSettings(BlockIssuanceCreditInput{},
			serix.TypeSettings{}.WithObjectType(uint8(ContextInputBlockIssuanceCredit))),
		)
		must(api.RegisterTypeSettings(RewardInput{},
			serix.TypeSettings{}.WithObjectType(uint8(ContextInputReward))),
		)

		must(api.RegisterTypeSettings(TxEssenceContextInputs{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithArrayRules(txEssenceV3ContextInputsArrRules),
		))

		must(api.RegisterInterfaceObjects((*txEssenceContextInput)(nil), (*CommitmentInput)(nil)))
		must(api.RegisterInterfaceObjects((*txEssenceContextInput)(nil), (*BlockIssuanceCreditInput)(nil)))
		must(api.RegisterInterfaceObjects((*txEssenceContextInput)(nil), (*RewardInput)(nil)))

		must(api.RegisterTypeSettings(UTXOInput{},
			serix.TypeSettings{}.WithObjectType(uint8(InputUTXO))),
		)

		must(api.RegisterTypeSettings(TxEssenceInputs{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithArrayRules(txEssenceV3InputsArrRules),
		))
		must(api.RegisterInterfaceObjects((*txEssenceInput)(nil), (*UTXOInput)(nil)))

		must(api.RegisterTypeSettings(TxEssenceOutputs{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithArrayRules(txEssenceV3OutputsArrRules),
		))

		must(api.RegisterTypeSettings(TxEssenceAllotments{},
			serix.TypeSettings{}.
				WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).
				WithArrayRules(txEssenceV3AllotmentsArrRules),
		))
		must(api.RegisterTypeSettings(TransactionCapabilitiesBitMask{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithMaxLen(1),
		))

		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*BasicOutput)(nil)))
		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*AccountOutput)(nil)))
		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*AnchorOutput)(nil)))
		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*DelegationOutput)(nil)))
		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*FoundryOutput)(nil)))
		must(api.RegisterInterfaceObjects((*TxEssenceOutput)(nil), (*NFTOutput)(nil)))
	}

	{
		must(api.RegisterTypeSettings(SignedTransaction{}, serix.TypeSettings{}.WithObjectType(uint8(PayloadSignedTransaction))))
		must(api.RegisterTypeSettings(Unlocks{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16).WithArrayRules(txV3UnlocksArrRules),
		))
		must(api.RegisterValidator(SignedTransaction{}, func(ctx context.Context, tx SignedTransaction) error {
			return tx.syntacticallyValidate()
		}))
		must(api.RegisterInterfaceObjects((*TxEssencePayload)(nil), (*TaggedData)(nil)))
	}

	{
		must(api.RegisterValidator(BlockIDs{}, func(ctx context.Context, blockIDs BlockIDs) error {
			return SliceValidator(blockIDs, LexicalOrderAndUniquenessValidator[BlockID]())
		}))
		must(api.RegisterTypeSettings(BlockIDs{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32),
		))
	}

	{
		must(api.RegisterValidator(TransactionIDs{}, func(ctx context.Context, transactionIDs TransactionIDs) error {
			return SliceValidator(transactionIDs, LexicalOrderAndUniquenessValidator[TransactionID]())
		}))
		must(api.RegisterTypeSettings(TransactionIDs{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32),
		))
	}

	{
		must(api.RegisterTypeSettings(HexOutputID(""), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte)))
	}

	{
		must(api.RegisterTypeSettings(BasicBlockBody{},
			serix.TypeSettings{}.WithObjectType(byte(BlockBodyTypeBasic))),
		)
	}

	{
		must(api.RegisterTypeSettings(ValidationBlockBody{},
			serix.TypeSettings{}.WithObjectType(byte(BlockBodyTypeValidation))),
		)
	}

	{
		must(api.RegisterInterfaceObjects((*BlockBody)(nil), (*BasicBlockBody)(nil)))
		must(api.RegisterInterfaceObjects((*BlockBody)(nil), (*ValidationBlockBody)(nil)))

		must(api.RegisterInterfaceObjects((*ApplicationPayload)(nil), (*SignedTransaction)(nil)))
		must(api.RegisterInterfaceObjects((*ApplicationPayload)(nil), (*TaggedData)(nil)))
		must(api.RegisterInterfaceObjects((*ApplicationPayload)(nil), (*CandidacyAnnouncement)(nil)))

		must(api.RegisterTypeSettings(Block{}, serix.TypeSettings{}))
		must(api.RegisterValidator(Block{}, func(ctx context.Context, block Block) error {
			return block.syntacticallyValidate()
		}))
	}

	{
		must(api.RegisterTypeSettings(Attestation{}, serix.TypeSettings{}))
		must(api.RegisterTypeSettings(Attestations{},
			serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte),
		))
	}

	{
		merklehasher.RegisterSerixRules[*APIByter[TxEssenceOutput]](api)
		merklehasher.RegisterSerixRules[Identifier](api)
	}

	return v3
}

func calculateRewards(protoParams ProtocolParameters) (initialRewards, finalRewards Mana, err error) {
	// final reward, after bootstrapping phase
	result, err := safemath.SafeMul(uint64(protoParams.TokenSupply()), protoParams.RewardsParameters().ManaShareCoefficient)
	if err != nil {
		return 0, 0, ierrors.Wrap(err, "failed to calculate target reward due to tokenSupply and RewardsManaShareCoefficient multiplication overflow")
	}

	result, err = safemath.SafeMul(result, uint64(protoParams.ManaParameters().GenerationRate))
	if err != nil {
		return 0, 0, ierrors.Wrapf(err, "failed to calculate target reward due to multiplication with generationRate overflow")
	}

	subExponent, err := safemath.SafeSub(protoParams.ManaParameters().GenerationRateExponent, protoParams.SlotsPerEpochExponent())
	if err != nil {
		return 0, 0, ierrors.Wrapf(err, "failed to calculate target reward due to generationRateExponent - slotsPerEpochExponent subtraction overflow")
	}

	finalRewardsUint := result >> subExponent

	// initial reward for bootstrapping phase
	initialReward, err := safemath.SafeMul(finalRewardsUint, protoParams.RewardsParameters().DecayBalancingConstant)
	if err != nil {
		return 0, 0, ierrors.Wrapf(err, "failed to calculate initial reward due to finalReward and DecayBalancingConstant multiplication overflow")
	}

	initialRewardsUint := initialReward >> uint64(protoParams.RewardsParameters().DecayBalancingConstantExponent)

	return Mana(initialRewardsUint), Mana(finalRewardsUint), nil
}
