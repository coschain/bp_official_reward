package types

type ManualType int

const (
	ManualTypeUnknown ManualType = 0
	ManualTypeSinglePeriod = 1
	ManualTypeBp   ManualType = 2
	ManualTypeVoters ManualType = 3
	ManualTypeLostRecord ManualType = 4
	ManualTypeFailRecord ManualType = 5
	ManualTypeLostVotersOfBp ManualType = 6
)

type ManualProcessModel struct {
	ManualProcessType ManualType
	Period uint64
	StartBlockNumber uint64
	EndBlockNumber uint64
	FailedRecIdList []string   //failed distribute records
	LostRecList  []*BpRewardRecord    //records not inserted to db
	NotDistributeVotersBp  []string     //bp which not distribute voters
}
