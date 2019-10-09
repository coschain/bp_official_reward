package types

const (
	DefaultPageSize = 10
	DefaultPageIndex = 1

	StatusSuccess = 200
	StatusWriteJsonError = 600
	StatusParamParseError = 601
	StatusGetDbError = 603
	StatusDbQueryError = 604
	StatusNotFoundError = 605
)

type UserRewardHistory struct {
	Voter  string
	Bp     string
	Amount string
	Time   string

}

type UserRewardHistoryResponse struct {
	Status    int
	Msg       string
	List      []*UserRewardHistory
}