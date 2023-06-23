package iotago

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/lo"
)

// splitUint64 splits a uint64 value into two uint64 that hold the high and the low double-word.
func splitUint64(value uint64) (valueHi uint64, valueLo uint64) {
	return value >> 32, value & 0x00000000FFFFFFFF
}

// mergeUint64 merges two uint64 values that hold the high and the low double-word into one uint64.
func mergeUint64(valueHi uint64, valueLo uint64) (value uint64) {
	return (valueHi << 32) | valueLo
}

// fixedPointMultiplication32Splitted does a fixed point multiplication using two uint64
// containing the high and the low double-word of the value.
// ATTENTION: do not pass factor that use more than 32bits, otherwise this function overflows.
func fixedPointMultiplication32Splitted(valueHi uint64, valueLo uint64, factor uint64, scale uint64) (uint64, uint64) {
	// multiply the integer part of the fixed-point number by the factor
	valueHi = valueHi * factor

	// the lower 'scale' bits of the result are extracted and shifted left to form the lower part of the new fraction.
	// the fractional part of the fixed-point number is multiplied by the factor and right-shifted by 'scale' bits.
	// the sum of these two values forms the new lower part (valueLo) of the result.
	valueLo = (valueHi&((1<<scale)-1))<<(32-scale) + (valueLo*factor)>>scale

	// the right-shifted valueHi and the upper 32 bits of valueLo form the new higher part (valueHi) of the result.
	valueHi = (valueHi >> scale) + (valueLo >> 32)

	// the lower 32 bits of valueLo form the new lower part of the result.
	valueLo = valueLo & 0x00000000FFFFFFFF

	// return the result as a fixed-point number composed of two 64-bit integers
	return valueHi, valueLo
}

// fixedPointMultiplication32 does a fixed point multiplication.
// ATTENTION: do not pass factor that use more than 32bits, otherwise this function overflows.
func fixedPointMultiplication32(value uint64, factor uint64, scale uint64) uint64 {
	valueHi, valueLo := splitUint64(value)
	return mergeUint64(fixedPointMultiplication32Splitted(valueHi, valueLo, factor, scale))
}

// ManaDecayProvider calculates the mana decay and mana generation
// using fixed point arithmetic and a precomputed lookup table.
type ManaDecayProvider struct {
	timeProvider *TimeProvider

	slotsPerEpochShiftFactor        uint64
	generationRate                  uint64 // the generation rate needs to be scaled by 2^-generationRateShiftFactor
	generationRateShiftFactor       uint64
	decayFactors                    []uint64 // the factors need to be scaled by 2^-decayFactorsShiftFactor
	decayFactorsLength              uint64
	decayFactorsShiftFactor         uint64
	decayFactorEpochsSum            uint64 // the factor needs to be scaled by 2^-decayFactorEpochsSumShiftFactor
	decayFactorEpochsSumShiftFactor uint64
}

func NewManaDecayProvider(
	timeProvider *TimeProvider,
	slotsPerEpochShiftFactor uint8,
	generationRate uint8,
	generationRateShiftFactor uint8,
	decayFactors []uint32,
	decayFactorsShiftFactor uint8,
	decayFactorEpochsSum uint32,
	decayFactorEpochsSumShiftFactor uint8) *ManaDecayProvider {

	return &ManaDecayProvider{
		timeProvider:                    timeProvider,
		slotsPerEpochShiftFactor:        uint64(slotsPerEpochShiftFactor),
		generationRate:                  uint64(generationRate),
		generationRateShiftFactor:       uint64(generationRateShiftFactor),
		decayFactors:                    lo.Map(decayFactors, func(factor uint32) uint64 { return uint64(factor) }),
		decayFactorsLength:              uint64(len(decayFactors)),
		decayFactorsShiftFactor:         uint64(decayFactorsShiftFactor),
		decayFactorEpochsSum:            uint64(decayFactorEpochsSum),
		decayFactorEpochsSumShiftFactor: uint64(decayFactorEpochsSumShiftFactor),
	}
}

// decay performs mana decay without mana generation.
func (p *ManaDecayProvider) decay(value Mana, epochIndexDiff EpochIndex) Mana {
	if value == 0 || epochIndexDiff == 0 || p.decayFactorsLength == 0 {
		// no need to decay if the epoch index didn't change or no decay factors were given
		return value
	}

	// split the value into two uint64 variables to prevent overflows
	valueHi, valueLo := splitUint64(uint64(value))

	// we keep applying the decay as long as epoch index diffs are left
	remainingEpochIndexDiff := epochIndexDiff
	for remainingEpochIndexDiff > 0 {
		// we can't decay more than the available epoch index diffs
		// in the lookup table in this iteration
		diffsToDecay := remainingEpochIndexDiff
		if diffsToDecay > EpochIndex(p.decayFactorsLength) {
			diffsToDecay = EpochIndex(p.decayFactorsLength)
		}
		remainingEpochIndexDiff -= diffsToDecay

		// slice index 0 equals epoch index diff 1
		decayFactor := p.decayFactors[diffsToDecay-1]

		// apply the decay and scale the resulting value (fixed-point arithmetics)
		valueHi, valueLo = fixedPointMultiplication32Splitted(valueHi, valueLo, decayFactor, p.decayFactorsShiftFactor)
	}

	// combine both uint64 variables to get the actual value
	return Mana(mergeUint64(valueHi, valueLo))
}

// generateMana calculates the generated mana.
func (p *ManaDecayProvider) generateMana(value BaseToken, slotIndexDiff SlotIndex) Mana {
	if slotIndexDiff == 0 || p.generationRate == 0 {
		return 0
	}

	return Mana(fixedPointMultiplication32(uint64(value), uint64(slotIndexDiff)*p.generationRate, p.generationRateShiftFactor))
}

// StoredManaWithDecay applies the decay to the given stored mana.
func (p *ManaDecayProvider) StoredManaWithDecay(storedMana Mana, slotIndexCreated SlotIndex, slotIndexTarget SlotIndex) (Mana, error) {
	epochIndexCreated := p.timeProvider.EpochsFromSlot(slotIndexCreated)
	epochIndexTarget := p.timeProvider.EpochsFromSlot(slotIndexTarget)

	if epochIndexCreated > epochIndexTarget {
		return 0, errors.Wrapf(ErrWrongEpochIndex, "the created epoch index was bigger than the target epoch index: %d > %d", epochIndexCreated, epochIndexTarget)
	}

	return p.decay(storedMana, epochIndexTarget-epochIndexCreated), nil
}

// PotentialManaWithDecay calculates the generated potential mana and applies the decay to the result.
func (p *ManaDecayProvider) PotentialManaWithDecay(deposit BaseToken, slotIndexCreated SlotIndex, slotIndexTarget SlotIndex) (Mana, error) {
	epochIndexCreated := p.timeProvider.EpochsFromSlot(slotIndexCreated)
	epochIndexTarget := p.timeProvider.EpochsFromSlot(slotIndexTarget)

	if epochIndexCreated > epochIndexTarget {
		return 0, errors.Wrapf(ErrWrongEpochIndex, "the created epoch index was bigger than the target epoch index: %d > %d", epochIndexCreated, epochIndexTarget)
	}

	epochIndexDiff := epochIndexTarget - epochIndexCreated
	switch epochIndexDiff {
	case 0:
		return p.generateMana(deposit, slotIndexTarget-slotIndexCreated), nil

	case 1:
		return p.decay(p.generateMana(deposit, p.timeProvider.SlotsBeforeNextEpoch(slotIndexCreated)), 1) + p.generateMana(deposit, p.timeProvider.SlotsSinceEpochStart(slotIndexTarget)), nil

	default:
		c := Mana(fixedPointMultiplication32(uint64(deposit), p.decayFactorEpochsSum, p.decayFactorEpochsSumShiftFactor+p.generationRateShiftFactor-p.slotsPerEpochShiftFactor))

		potentialMana_n := p.decay(p.generateMana(deposit, p.timeProvider.SlotsBeforeNextEpoch(slotIndexCreated)), epochIndexDiff)
		potentialMana_n_1 := p.decay(c, epochIndexDiff-1)
		potentialMana_0 := p.generateMana(deposit, p.timeProvider.SlotsSinceEpochStart(slotIndexTarget)) + c

		return potentialMana_n - potentialMana_n_1 + potentialMana_0, nil
	}
}

// RewardsWithDecay applies the decay to the given stored mana.
func (p *ManaDecayProvider) RewardsWithDecay(rewards Mana, rewardEpochIndex EpochIndex, epochIndexClaimed EpochIndex) (Mana, error) {
	if rewardEpochIndex > epochIndexClaimed {
		return 0, errors.Wrapf(ErrWrongEpochIndex, "the reward epoch index was bigger than the claiming epoch index: %d > %d", rewardEpochIndex, epochIndexClaimed)
	}

	return p.decay(rewards, epochIndexClaimed-rewardEpochIndex), nil
}
