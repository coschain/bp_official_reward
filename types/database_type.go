package types

import "time"

const (
	ProcessingStatusDefault = 0
	ProcessingStatusSuccess = 1
	ProcessingStatusFail    = 2
	ProcessingStatusNotTransfer = 3
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
	Period int64   `gorm:"not null"`
	Amount string  `gorm:"not null"`
	Bp     string `gorm:"not null"`
	Voter  string `gorm:"not null"`
	TransferHash string `gorm:"not null"`
	Time   int64   `gorm:"not null"`
	Status  int     `gorm:"not null"`
	Vest   uint64    `gorm:"not null"`
	TotalBlockReward string `gorm:"not null"`
	SingleBlockReward string `gorm:"not null"`
	TotalVoterVest   string  `gorm:"not null"`
	CreatedBlockNumber uint64 `gorm:"not null"`
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