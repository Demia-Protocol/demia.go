package iotago

import (
	"context"
	"fmt"

	"github.com/iotaledger/hive.go/ierrors"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	CommitmentIDLength = SlotIdentifierLength
)

type CommitmentID = SlotIdentifier

var EmptyCommitmentID = CommitmentID{}

type Commitment struct {
	Version          Version      `serix:"0,mapKey=version"`
	Index            SlotIndex    `serix:"1,mapKey=index"`
	PrevID           CommitmentID `serix:"2,mapKey=prevId"`
	RootsID          Identifier   `serix:"3,mapKey=rootsId"`
	CumulativeWeight uint64       `serix:"4,mapKey=cumulativeWeight"`
}

func NewCommitment(version Version, index SlotIndex, prevID CommitmentID, rootsID Identifier, cumulativeWeight uint64) *Commitment {
	return &Commitment{
		Version:          version,
		Index:            index,
		PrevID:           prevID,
		RootsID:          rootsID,
		CumulativeWeight: cumulativeWeight,
	}
}

func NewEmptyCommitment(version Version) *Commitment {
	return &Commitment{
		Version: version,
	}
}

func (c *Commitment) ID() (CommitmentID, error) {
	data, err := CommonSerixAPI().Encode(context.TODO(), c)
	if err != nil {
		return CommitmentID{}, ierrors.Errorf("can't compute commitment ID: %w", err)
	}

	return SlotIdentifierRepresentingData(c.Index, data), nil
}

func (c *Commitment) StateID() Identifier {
	return IdentifierFromData(lo.PanicOnErr(c.MustID().Bytes()))
}

func (c *Commitment) Type() StateType {
	return InputCommitment
}

func (c *Commitment) MustID() CommitmentID {
	id, err := c.ID()
	if err != nil {
		panic(err)
	}

	return id
}

func (c *Commitment) Equals(other *Commitment) bool {
	return c.MustID() == other.MustID() &&
		c.Version == other.Version &&
		c.Index == other.Index &&
		c.PrevID == other.PrevID &&
		c.RootsID == other.RootsID &&
		c.CumulativeWeight == other.CumulativeWeight
}

func (c *Commitment) String() string {
	return fmt.Sprintf("Commitment{\n\tIndex: %d\n\tPrevID: %s\n\tRootsID: %s\n\tCumulativeWeight: %d\n}",
		c.Index, c.PrevID, c.RootsID, c.CumulativeWeight)
}

func (c *Commitment) Size() int {
	return serializer.OneByte +
		SlotIndexLength +
		CommitmentIDLength +
		IdentifierLength +
		serializer.UInt64ByteSize
}
