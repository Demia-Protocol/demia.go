package iotago

// CandidacyAnnouncement is a payload which is used to indicate candidacy for committee selection for the next epoch.
type CandidacyAnnouncement struct {
}

func (u *CandidacyAnnouncement) Clone() Payload {
	return &CandidacyAnnouncement{}
}

func (u *CandidacyAnnouncement) PayloadType() PayloadType {
	return PayloadCandidacyAnnouncement
}

func (u *CandidacyAnnouncement) Size() int {
	// PayloadType
	return 0
}

func (u *CandidacyAnnouncement) WorkScore(workScoreStructure *WorkScoreStructure) (WorkScore, error) {
	// we account for the network traffic only on "Payload" level
	// TODO: is the work score correct?
	return workScoreStructure.DataByte.Multiply(u.Size())
}
