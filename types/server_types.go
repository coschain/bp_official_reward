package types

const (
	DefaultPageSize = 10
	DefaultPageIndex = 1

	StatusSuccess = 200
	StatusWriteJsonError = 600
	StatusParamParseError = 601
	StatusLackParamError = 602
	StatusGetDbError = 603
	StatusDbQueryError = 604
	StatusNotFoundError = 605
	StatusGetLatestPeriodError = 606
	StatusGetPeriodNotFoundError = 607
	StatusGetBlockLogError  = 608
	StatusCalcCreatedBlockError = 609
	StatusGetLibError = 610
	StatusGetAllVoterVestError = 611
	StatusGetTotalVoterCountError = 612
	StatusGetMaxROIError = 613
	StatusConvertRewardError = 614
)

type BaseResponse struct {
	Status    int
	Msg       string
}

type UserRewardHistory struct {
	Voter  string
	Bp     string
	Amount string
	Time   string
	Period string
	BlockNumber string

}

type UserRewardHistoryResponse struct {
	BaseResponse
	List      []*UserRewardHistory
}

type RewardRecord struct {
	BpName  string         // account name of block producer
	GenBlockCount string   // number of blocks generated this period
	TotalReward  string   // all the rewards for a period
	IsDistributable bool  // is support distribute
	RewardRate   string  //bp reward rate
	VotersVest    string  //total vest of all voters
	EveryThousandRewards string  //rewards per 1000 vest
	AnnualizedROI  string   //annualized rate

}

type AnnualizedInfo struct {
	IsDistributable bool
	TotalReward string   // cold start + ecological reward
	RewardRate   string  //bp reward rate
	EveryThousandRewards string  //rewards per 1000 vest
	AnnualizedROI  string   //annualized rate
}

type RewardInfo struct {
	Period  int              // current period
	StartBlockNumber string  //start block number of a period
	StartBlockTime string    //start block time of a period
	EndBlockNumber string    //end block number of a period
	EndBlockTime   string    // end block time of a period
	DistributeTime string    // distribute time of this period
	List           []*RewardRecord //reward record list
}

type RewardInfoResponse struct {
	BaseResponse
	List      []*RewardInfo
}

type EstimatedRewardInfo struct {
	BpName  string  //account name of block producer
	GenBlockCount  string  //account name of block producer
	AccumulatedReward string //accumulated block reward
	EstimatedTotalReward string //estimated total block reward
	RewardRate   string  //bp reward rate
	EstimatedVotersVest  string // estimated total vest of all voters
	EstimatedThousandRewards string //estimated rewards per 1000 vest
	EstimatedAnnualizedROI  string //estimated annualized rate
	IsDistributable bool  // is support distribute

}

type EstimatedRewardInfoModel struct {
	StartBlockNumber string  //start block number of a period
	EndBlockNumber string    //end block number of a period
	DistributeTime string    // distribute time of this period
	UpdateTime  string      // estimated reward info update time
	List   []*EstimatedRewardInfo
}

type EstimatedRewardInfoResponse struct {
	BaseResponse
	Info      *EstimatedRewardInfoModel
}

type EstimatedVoterRewardResponse struct {
	RewardInfo EstimatedRewardInfo
}

type HistoricalVotingData struct {
	VotersNumber  string
	MaxROI     string
}

type HistoricalVotingDataResponse struct {
	BaseResponse
	Info      *HistoricalVotingData
}

type OnePeriodBlockInfo struct {
	StartBlockNum uint64
	StartBlockTime int64
	EndBlockNum uint64
	EndBlockTime int64
	DistributeTime int64
}

func (info *RewardInfo) Len() int {
	return len(info.List)
}

func (info *RewardInfo) Less(i, j int) bool {
	return  info.List[i].AnnualizedROI > info.List[j].AnnualizedROI
}

func (info *RewardInfo) Swap(i, j int) {
    info.List[i], info.List[j] = info.List[j], info.List[i]
}

func (r *EstimatedRewardInfoModel) Len() int {
	return len(r.List)
}

func (r *EstimatedRewardInfoModel) Less(i, j int) bool {
	return  r.List[i].EstimatedAnnualizedROI > r.List[j].EstimatedAnnualizedROI
}

func (r *EstimatedRewardInfoModel) Swap(i, j int)  {
	 r.List[i], r.List[j] = r.List[j], r.List[i]
}