package distribute

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"github.com/coschain/contentos-go/common/constants"
	"github.com/robfig/cron"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"math/big"
	"sync"
	"time"
)

const (
	MainNetStartTimeStamp = 1569380400 //20190925 11am
    YearDay = 365
    FirstYearTotalReward =  50850000
    SecondYearTotalReward = 32100000
    ThirdYearTotalReward  = 25500000
    FourthYearTotalReward = 21900000
    FifthYearTotalReward  = 19650000
    RewardRate = 0.8
    //DistributeInterval = 7 * 86400 // a week 7 day
	DistributeInterval = 3 * 60
)

var (
	sv  *RewardDistributeService
	svOnce sync.Once
)

type RewardDistributeService struct {
	logger  *logrus.Logger
	cron    *cron.Cron
	isHandling bool
}

func NewDistributeService() *RewardDistributeService {
	cron := cron.New()
	return &RewardDistributeService{
		logger: logs.GetLogger(),
		cron: cron,
	}
}

func StartDistributeService() error {
	sv = NewDistributeService()
	spec := "0 0 11 ? * WED"  //distribute every Wednesday 11.00 am
	err := sv.cron.AddFunc(spec, sv.startDistribute)
	if err != nil {
		sv.logger.Errorf("StartDistributeService: fail to start distribute service, the error is %v", err)
		return err
	}
	sv.cron.Start()
	return nil
}

func StopDistributeService() {
	if sv != nil && sv.cron != nil {
		sv.cron.Stop()
	}
}

func (sv *RewardDistributeService) startDistribute()  {
	sv.logger.Infof("Start this round reward distribute")
	if sv.isHandling {
		sv.logger.Infoln("last round distribute not finish")
		return
	}
	t := time.Now()
	curTime := time.Now().Unix()
	lastPeriod := curTime - DistributeInterval
	sv.logger.Infof("startDistribute: start distribute of this week from:%v to:%v", lastPeriod, curTime)
	//calculate single block reward of this year
	singleReward,curYear := GetSingleBlockReward(t)
	if singleReward == nil {
		sv.logger.Errorf("startDistribute: not support distribute for year %v", curYear)
		return
	}
	for _,bp := range OfficialBpList {
		voterList,err := db.GetAllRewardedVotersOfPeriodByBp(bp, lastPeriod, curTime)
		if err != nil {
			sv.logger.Errorf("startDistribute: Fail to get reward voters of bp:%v, the error is %v", bp, err)
		} else {
			sv.logger.Infof("startDistribute: this round voters number of bp:%v is %v", bp, len(voterList))
			sv.logger.Infof("startDistribute: single block reward of year:%v is %v", curYear, singleReward.String())
			//1.calculate bp reward of this week
			generatedBlkNum,err :=  db.CalcBpGeneratedBlockNum(bp, lastPeriod, curTime)
			sv.logger.Infof("startDistribute: generated block of bp:%v is %v", bp, generatedBlkNum)
			if err != nil {
				sv.logger.Errorf("startDistribute: Fail to get generated block of bp:%v, the error is %v", bp, err)
			} else {
				//calculate total block reward of this bp
				bigNum := new(big.Int).SetUint64(generatedBlkNum)
				totalReward := singleReward.Mul(decimal.NewFromBigInt(bigNum, 0))
				sv.logger.Infof("startDistribute: total block reward of bp:%v is %v", bp, totalReward.String())
				//calculate actually distributed rewards (total reward * rate)
				distributeReward := totalReward.Mul(decimal.NewFromFloat(RewardRate))
				sv.logger.Infof("startDistribute: total distribute block reward of bp:%v is %v", bp, distributeReward.String())
				//calculate all voter's total vest
				totalVest := getTotalVestOfAccounts(voterList)
				sv.logger.Infof("startDistribute: total vest of all voters of bp:%v is %v", bp, totalVest.String())
				for _,acct := range voterList {
					bigVal := new(big.Int).SetUint64(acct.Vest)
					vest := decimal.NewFromBigInt(bigVal, 0)
					actualReward,_ := distributeReward.Mul(vest).QuoRem(totalVest, 6)
					distributeReward := actualReward.Mul(decimal.NewFromFloat(constants.COSTokenDecimals))
					transferReward := new(big.Int)
					transferReward.SetString(distributeReward.String(), 10)
					sv.logger.Infof("startDistribute: calculate voter:%v reward is %v, actually reward is %v", acct.Name, distributeReward, transferReward)

					//create reward record
					rewardRec := &types.BpRewardRecord{
						Id: utils.GenerateId(t, acct.Name, bp, "reward"),
						Period: getPeriod(curTime),
						Amount: actualReward.String(),
						Bp: bp,
						Voter: acct.Name,
						Time: curTime,
						Status: types.ProcessingStatusDefault,
						Vest: acct.Vest,
						TotalBlockReward: totalReward.String(),
						SingleBlockReward: singleReward.String(),
						TotalVoterVest: totalVest.String(),
						CreatedBlockNumber: generatedBlkNum,
					}
					// insert reward record to db
					senderAcct,senderPrivkey := config.GetMainNetCosSenderInfo()
                    txHash,sigedTx,err := utils.GenTransferToVestSignedTx(senderAcct, senderPrivkey, acct.Name, transferReward, "")
                    isTransfer := true
                    if err != nil {
                    	sv.logger.Errorf("startDistribute: fail to generate transfer vest to %v tx, the error is %v", acct.Name, err)
                    	rewardRec.Status = types.ProcessingStatusNotTransfer
						isTransfer = false
					} else {
						rewardRec.TransferHash = txHash
					}
                    err = db.InsertRewardRecord(rewardRec)
                    if err != nil {
                    	sv.logger.Errorf("startDistribute: fail to insert reward record to db, the error is %v", err)
					} else if isTransfer{
						//transfer to vest for reward
						_,err = utils.TransferVestBySignedTx(sigedTx)
						if err != nil {
							sv.logger.Errorf("startDistribute: fail to transfer vest to %v for bp:%v vote, the error is %v", acct.Name, bp, err)
						}
					}
				}

			}
		}
	}
	sv.logger.Infoln("startDistribute: finish this round distribute")
}

func getEveryYearRewardMap() map[int64]int64 {
	return map[int64]int64 {
		1: FirstYearTotalReward,
		2: SecondYearTotalReward,
		3: ThirdYearTotalReward,
		4: FourthYearTotalReward,
		5: FifthYearTotalReward,
	}
}

func GetSingleBlockReward(t time.Time) (*decimal.Decimal,int64) {
	timestamp := t.Unix()
	diff := timestamp - MainNetStartTimeStamp
	rewardMap := getEveryYearRewardMap()
	year := YearDay * 86400
	curYear := (diff / (int64(year))) + 1
	if reward,ok := rewardMap[curYear]; ok {
		 bigReward := new(big.Int).SetInt64(reward)
		 totalDecimal := decimal.NewFromBigInt(bigReward, 0)
		 bigYear := new(big.Int).SetInt64(int64(year))
		 singleReward,_ := totalDecimal.QuoRem(decimal.NewFromBigInt(bigYear, 0), 6)
		 return &singleReward,curYear
	}
	return nil, curYear
}

func getTotalVestOfAccounts(list []*types.AccountInfo) decimal.Decimal {
	total := decimal.New(0, 0)
	for _,acct := range list {
		bigVal := new(big.Int).SetUint64(acct.Vest)
		total = total.Add(decimal.NewFromBigInt(bigVal, 0))
	}
	return total
}

func getPeriod(t int64) int64 {
	return (t-MainNetStartTimeStamp)/DistributeInterval
}