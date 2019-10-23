package types

import (
	"time"
)

const (
	ProcessingStatusDefault = 0   //default status
	ProcessingStatusSuccess = 1   //transfer success
	ProcessingStatusFail    = 2   //transfer fail
	ProcessingStatusNotNeedTransfer = 3 //not need to transfer(transfer amount is 0 or voter's vest is invalid(less than 10 VEST))
	ProcessingStatusGenTxFail = 4  //generate transfer to vest hash fail
	ProcessingStatusPending = 5  // use to mark the only statistics not distribute reward by service in the early stage

	RewardTypeToVoter = 0   //distribute reward to bp
	RewardTypeToBp = 1      //distribute reward to voter
)

type AccountInfo struct {
	AccountId   string       `gorm:"primary_key"`
	Time     int64           `gorm:"not null"`
	Name  string             `gorm:"not null"`
	Balance uint64			 `gorm:"not null"`
	Vest uint64				 `gorm:"not null"`
	StakeVestFromMe uint64	 `gorm:"not null"`
}

type BpVoteRecord struct {
	VoteId    string            `gorm:"primary_key"`
	BlockHeight uint64			`gorm:"not null"`
	BlockTime time.Time         `gorm:"not null"`
	Voter string				`gorm:"not null"`
	Producer string				`gorm:"not null"`
	Cancel bool                 `gorm:"not null"`
	Time     int64              `gorm:"not null"`
}

type BpVoteRelation struct {
	VoteId   string  `gorm:"primary_key"`
	Voter    string  `gorm:"not null"`
	Producer string  `gorm:"not null"`
	Time     int64   `gorm:"not null"`
}

type BpRewardRecord struct {
	Id  string  `gorm:primary_key`
	Period uint64
	RewardType int  `gorm:"not null"`
	RewardAmount string  `gorm:"not null"`
	RewardRate float64 `gorm:"not null"`
	ColdStartRewardRate float64 `gorm:"not null"`
	Bp     string `gorm:"not null"`
	Voter  string `gorm:"not null"`
	TransferHash string `gorm:"not null"`
	Time   int64   `gorm:"not null"`
	Status  int     `gorm:"not null"`
	Vest   string    `gorm:"not null"`
	TotalBlockReward string `gorm:"not null"`
	SingleBlockReward string `gorm:"not null"`
	SingleColdStartReward   string  `gorm:"not null"`
	TotalVoterVest   string  `gorm:"not null"`
	CreatedBlockNumber uint64 `gorm:"not null"`
	DistributeBlockNumber  uint64 `gorm:"not null"`
	AnnualizedRate  float64  `gorm:"not null"`
}

type CosTrxInfo struct {
	Id   uint64
	TrxId string
	BlockHeight uint64
	BlockTime uint64
	Invoice  string
	Operations string
	BlockId   string
	Creator  string
}


type CosTxInvoice struct {
	Status uint32 `json:"status"`
	Cpu_usage uint64 `json:"cpu_usage"`
	Net_usage uint64 `json:"net_usage"`
}

type LibInfo struct {
	Lib  uint64
	LastCheckTime uint32
}

type BpBlockStatistics struct {
	TotalCount  uint64
	BlockProducer string
}


