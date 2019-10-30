package distribute

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"errors"
	"fmt"
	"github.com/coschain/contentos-go/app/plugins"
	"github.com/coschain/contentos-go/common/constants"
	"github.com/coschain/contentos-go/prototype"
	"github.com/robfig/cron"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	MainNetStartTimeStamp = 1569380400 //20190925 11am
    YearDay = 365
    RewardRate float64 = 0.79  //cold start reward rate
    //cold start reward
	ColdStartRewardMaxYear = 5
	//Ecological reward
	EcologicalRewardMaxYear = 12
	YearBlkNum = YearDay * 86400

	TransferTypeDefault = 0
	TransferTypeInvalideVoter = 1 //voter's valid vest is less than utils.MinVoterDistributeVest
	TransferTypePending = 2
	TransferTypeOfficialBp = 3 // official distribute bp (not need distribute reward to these bp)
)

var (
	sv  *RewardDistributeService
	svOnce sync.Once
	ecologicalRewardMap = map[int]uint64 {
		1: 13440000,
		2: 26880000,
		3: 40320000,
		4: 53760000,
		5: 67200000,
		6: 80640000,
		7: 94080000,
		8: 107520000,
		9: 120960000,
	   10: 134400000,
	   11: 147840000,
	   12: 162960000,
	}

	coldStartRewardMap = map[int]uint64 {
		1: 50850000,
		2: 32100000,
		3: 25500000,
		4: 21900000,
		5: 19650000,
	}

	cacheSv  *cacheService
	isEstimating bool
)

type RewardDistributeService struct {
	logger  *logrus.Logger
	cron    *cron.Cron
	isHandling bool
	stopCh     chan bool
}

type bpRewardInfo struct {
	rewardRecord *types.BpRewardRecord
	VoterList []*types.AccountInfo
}

type cacheService struct {
	estimateRewardInfo *types.EstimatedRewardInfoModel
	stopCh     chan bool
	logger  *logrus.Logger
	isEstimating bool
}

func CacheServiceInstance() *cacheService {
	svOnce.Do(func() {
		if cacheSv == nil {
			cacheSv = &cacheService{
				logger: logs.GetLogger(),
			}
		}
	})
	return cacheSv
}

func (c *cacheService) StartCacheSerVice() {
	ticker := time.NewTicker(time.Duration(config.CacheTimeInterval))
	go func() {
		for {
			select {
			case <- ticker.C:
				c.cacheEstimatedRewardInfo()
			case <- c.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (c *cacheService) StopCacheSerVice() {
	c.stopCh <- true
	close(sv.stopCh)
}

func (c *cacheService) cacheEstimatedRewardInfo()  {
	c.logger.Infoln("start this round cacheEstimatedRewardInfo")
	if c.isEstimating {
		c.logger.Infoln("last round estimate is not finish")
		return
	}

	c.isEstimating = true
	info,err,_ := estimateCurrentPeriodReward()
	if err != nil {
		c.logger.Errorf("Fail to estimate reward info, the error is %v", err)
	}
	c.estimateRewardInfo = info
	c.isEstimating = false
	c.logger.Infoln("finish this round cacheEstimatedRewardInfo")
}

func (c *cacheService) GetEstimatedRewardInfo() *types.EstimatedRewardInfoModel {
	return c.estimateRewardInfo
}

func (c *cacheService) SetEstimatedRewardInfo(info *types.EstimatedRewardInfoModel) {
	if info != nil {
		c.estimateRewardInfo = info
	}

}

func NewDistributeService() *RewardDistributeService {
	return &RewardDistributeService{
		logger: logs.GetLogger(),
	}
}

func (sv* RewardDistributeService)StartDistributeService() error {
	ticker := time.NewTicker(time.Duration(config.DistributeTimeInterval))
	go func() {
		for {
			select {
			case <- ticker.C:
				sv.handleDistribute()
			case <- sv.stopCh:
				ticker.Stop()
				CacheServiceInstance().StopCacheSerVice()
				return
			}
		}
	}()
	CacheServiceInstance().StartCacheSerVice()
	return nil
}

func (sv *RewardDistributeService) StopDistributeService() {
	sv.stopCh <- true
	close(sv.stopCh)
}

func (sv *RewardDistributeService) handleDistribute()  {
	//distribute rule: we use 86400 * 7 blocks as a calculate and distribute cycle, Regularly obtain the
	// latest irreversible block. If it is found that the block with the current latest block is greater
	// than or equal to 86400*7, then this week's statistical settlement is performed.

	if sv.isHandling {
		sv.logger.Infof("handleDistribute: last round distribute not finish")
		return
	}
	sv.isHandling = true
	// get latest irreversible block
	curTime := time.Now()
	t := curTime.Unix()
	sv.logger.Infof("handleDistribute: it's time to handle distribute on time %v", t)
	lib,err := db.GetLib()
	if err != nil {
		sv.logger.Errorf("handleDistribute: fail to get latest lib on time %v, the error is %v", t, err)
		sv.isHandling = false
		return
	}
	//calculate current period
	curPeriod := GetPeriodByBlockNum(lib)
	sv.logger.Infof("handleDistribute: curPeriod is %v, lib is %v", curPeriod, lib)
    if curPeriod < 1 {
		sv.isHandling = false
		return
	} else {
		sBlkNum := config.ServiceStarPeriodBlockNum +  (curPeriod - 1) * config.DistributeInterval
		eBlkNum := config.ServiceStarPeriodBlockNum + curPeriod * config.DistributeInterval
		//judge is need to distribute for this period
		latestPeriod,err := db.GetLatestDistributedPeriod(true)
		if err != nil {
			sv.logger.Errorf("handleDistribute: fail to get latest distributed period, the error is %v", err)
			sv.isHandling = false
			return
		}

		if latestPeriod + 1 < curPeriod {
			// last period has not distribute
			// distribute last period reward
			lastPeriod := curPeriod - 1
			s := config.ServiceStarPeriodBlockNum + (lastPeriod-1) * config.DistributeInterval
			e := config.ServiceStarPeriodBlockNum + lastPeriod * config.DistributeInterval
			sv.logger.Infof("handleDistribute: distribute reward of last period:%v, current period is %v", lastPeriod, curPeriod)
			sBlkTime,eBlkTime,err := sv.getPeriodRangeBlockTime(s, e)
			if err != nil {
				sv.logger.Infof("handleDistribute: distribute reward of last period:%v, fail to get start and end block time,the error is %v", lastPeriod, err)
			} else {
				sv.startDistribute(lastPeriod, sBlkTime, eBlkTime ,s, e)
				latestPeriod = curPeriod - 1
			}

		}

		if latestPeriod + 1 == curPeriod {
			//need distribute this period reward
			sBlkTime,eBlkTime,err := sv.getPeriodRangeBlockTime(sBlkNum, eBlkNum)
			if err != nil {
				sv.logger.Infof("handleDistribute: distribute current period:%v, fail to get start and end block time,the error is %v", curPeriod, err)
			} else {
				sv.startDistribute(curPeriod, sBlkTime, eBlkTime ,sBlkNum, eBlkNum)
				latestPeriod = curPeriod - 1
			}
		}
	}
	sv.isHandling = false
}

func (sv *RewardDistributeService) startDistribute(period uint64, sTime time.Time, eTime time.Time, startBlk uint64, endBlk uint64)  {
	sv.logger.Infof("Start this round reward distribute of period %v", period)

	t := time.Now()
	curTime := t.Unix()
	sv.logger.Infof("startDistribute: start distribute of this week,the block number range is from:%v to:%v", startBlk, endBlk)
	//calculate single block reward of cold start this year
	curYear := getYearByPeriod(period)
	coldStartSingleReward := CalcSingleBlockRewardOfColdStartByPeriod(period)
	if coldStartSingleReward == nil {
		sv.logger.Errorf("startDistribute: not support distribute for year %v", curYear)
		return
	}
	//1. distribute reward to bp
	bpSingleReward := CalcSingleBlockRewardByPeriod(period)
	bpRewardInfoList,_ := sv.startDistributeToBp(period, sTime, eTime, t, bpSingleReward, *coldStartSingleReward ,startBlk, endBlk)
	//2. distribute reward to all voters
	for _,bp := range config.OfficialBpList {
		sv.logger.Infof("startDistribute: single block reward of year:%v is %v", curYear, coldStartSingleReward.String())
		//1.calculate bp reward of this week
		var (
			generatedBlkNum uint64
			voterList []*types.AccountInfo
			err error
		)
		totalVest := decimal.New(0,0)
		isNeedCalcBlocks := true
		isBlkCntValid := true
		isNeedCalcTotalVest := true
		if len(bpRewardInfoList) > 0 {
			// get blocks count from bpBlkInfoList directly
			for _,info := range bpRewardInfoList {
				bpReward := info.rewardRecord
				if bpReward.Bp == bp {
					generatedBlkNum = bpReward.CreatedBlockNumber
					isNeedCalcBlocks = false
					if info.rewardRecord.TotalVoterVest != "-1" {
						totalVest,err = decimal.NewFromString(bpReward.TotalVoterVest)
						if err == nil {
							isNeedCalcTotalVest = false
							voterList = info.VoterList
						}
					}
					break
				}
			}
		}
		if isNeedCalcBlocks {
			// calculate total block number generated by bp
			generatedBlkNum,err =  db.CalcBpTotalBlockNumOfRange(bp, startBlk, endBlk)
			if err != nil {
				sv.logger.Errorf("startDistribute: Fail to get generated block of bp:%v, the error is %v", bp, err)
				isBlkCntValid = false
			}
		}
		//calculate all voter's total vest of bp
		if isNeedCalcTotalVest {
			voterList,err = db.GetAllRewardedVotersOfPeriodByBp(bp, sTime.Unix(), eTime.Unix(), startBlk, endBlk)
			if err != nil {
				sv.logger.Errorf("startDistribute: Fail to get all voters who can get reward from bp:%v, the error is %v", bp, err)
				continue
			} else {
				//calculate all voter's total vest, filter distribute bp if it has voted to self
				totalVest = getTotalVestOfVoters(voterList, true)
			}
		}
		sv.logger.Infof("startDistribute: this round voters number of bp:%v is %v, total vest of all voters is %v", bp, len(voterList), totalVest.String())

		if isBlkCntValid {
				sv.logger.Infof("startDistribute: generated block of bp:%v is %v", bp, generatedBlkNum)
				//calculate total block reward of this bp
				totalReward := calcTotalRewardOfBpOnOnePeriod(*coldStartSingleReward, generatedBlkNum)
				sv.logger.Infof("startDistribute: total block reward of bp:%v is %v", bp, totalReward.String())
				//calculate actually distributed rewards (total reward * rate)
				distributeReward := totalReward
				sv.logger.Infof("startDistribute: total distribute block reward of bp:%v is %v", bp, distributeReward.String())
				for _,acct := range voterList {
					if !checkIsValidVoterVest(acct.Vest) || acct.Name == bp{
						//not distribute reward to voter whose vest is less than utils.MinVoterDistributeVest
						//not distribute reward to official bp if official bp vote it self
						continue
					}
					bigVal := new(big.Int).SetUint64(acct.Vest)
					vest := decimal.NewFromBigInt(bigVal, 0)
					actualReward :=  calcActualReward(distributeReward, vest, totalVest)
					distributeReward := actualReward
					transferReward := new(big.Int)
					transferReward.SetString(distributeReward.String(), 10)
					distributeReward,_ = distributeReward.QuoRem(decimal.NewFromFloat(constants.COSTokenDecimals), 6)
					sv.logger.Infof("startDistribute: calculate voter:%v's reward is %v, actually reward is %v, vest is %v", acct.Name, distributeReward, transferReward, utils.CalcActualVest(acct.Vest))

					//create reward record
					rewardRec := &types.BpRewardRecord{
						Id: utils.GenerateId(t, acct.Name, bp, "reward to voter"),
						Period: period,
						RewardType: types.RewardTypeToVoter,
						RewardAmount: distributeReward.String(),
						RewardRate: RewardRate,
						ColdStartRewardRate: RewardRate,
						Bp: bp,
						Voter: acct.Name,
						Time: curTime,
						Status: types.ProcessingStatusDefault,
						Vest: utils.CalcActualVest(acct.Vest),
						TotalBlockReward: totalReward.String(),
						SingleBlockReward: coldStartSingleReward.String(),
						SingleColdStartReward: coldStartSingleReward.String(),
						TotalVoterVest: totalVest.String(),
						CreatedBlockNumber: generatedBlkNum,
						DistributeBlockNumber: endBlk,
						AnnualizedRate: calcAnnualizedROI(generatedBlkNum, *coldStartSingleReward, RewardRate, totalVest.String()),
					}
					disType := TransferTypePending
					if bigVal.Uint64() < utils.MinVoterDistributeVest {
						//if voter's vest is less than 10 vest, not need to distribute reward to it
						disType = TransferTypeInvalideVoter
					}
					sv.sendRewardToAccount(rewardRec, transferReward, disType) 
				}
		}

	}
	sv.logger.Infof("startDistribute: finish this round distribute of period:%v", period)
}


//
// distribute reward to all bp that have generated blocks successfully
//
func (sv *RewardDistributeService) startDistributeToBp(period uint64,sTime time.Time, eTime time.Time, distributeTime time.Time, singleBlkReward decimal.Decimal, singleBlkColdStartReward decimal.Decimal, startBlk uint64, endBlk uint64) ([]*bpRewardInfo, error){
	sv.logger.Infof("startDistributeToBp: start distribute reward to bp on block range(from:%v,to:%v)", startBlk, endBlk)
	//1. calculate all bp's total generated blocks
	statistics,err := db.CalcBpGeneratedBlocksOnOnePeriod(startBlk, endBlk)
	if err != nil {
		sv.logger.Errorf("startDistributeToBp: fail to calculate bp block statistics on period:%v , the error is %v", period ,err)
		return nil, err
	}
	count := len(statistics)
	eTimeStamp := eTime.Unix()
	sTimeStamp  := sTime.Unix()
	sv.logger.Infof("startDistributeToBp: start time is %v, end time is %v", sTimeStamp, eTimeStamp)
	var list []*bpRewardInfo
	if count < 1 {
		sv.logger.Errorf("startDistributeToBp: block statistics is empty on period:%v", period)
	} else {
		sv.logDistributeBpInfo(period, statistics)
		for _,data := range statistics {
			// calculate total reward of bp(cold start + ecological reward)
			totalAmount := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, data.TotalCount)
			sv.logger.Infof("startDistributeToBp: total block reward of bp:%v is %v", data.BlockProducer, totalAmount.String())
			//we just distribute cold start reward for bp,because Ecological Reward distributed by cos chain
			distributeReward := calcTotalRewardOfBpOnOnePeriod(singleBlkColdStartReward, data.TotalCount)
			distributeRewardStr := distributeReward.String()
			sv.logger.Infof("startDistributeToBp: actual distribute reward to bp:%v is %v", data.BlockProducer, distributeRewardStr)
			// Multiply COSTokenDecimals
			distributeReward = distributeReward.Mul(decimal.NewFromFloat(constants.COSTokenDecimals))
			bigAmount := new(big.Int)
			bigAmount.SetString(distributeReward.String(), 10)
			voterList,err := db.GetAllRewardedVotersOfPeriodByBp(data.BlockProducer, sTimeStamp, eTimeStamp, startBlk, endBlk)
			totalVest := decimal.New(-1, 0)
			if err != nil {
				sv.logger.Errorf("startDistributeToBp: fail to get all voters of bp:%v on this period, the error is %v", data.BlockProducer, err)
			} else {
				//filter distribute bp if it has voted self
				totalVest = getTotalVestOfVoters(voterList, true)
			}
			sv.logger.Infof("startDistributeToBp: bp:%v's totalVest is %v,voters count is %v", data.BlockProducer, totalVest.String(), len(voterList))
			//send reward to bp
			rewardRate := RewardRate
			if !CheckIsDistributableBp(data.BlockProducer) {
				rewardRate = 1
			}
			rec := &types.BpRewardRecord{
				Id: utils.GenerateId(distributeTime, data.BlockProducer, data.BlockProducer, "Reward to bp"),
				Period: period,
				RewardType: types.RewardTypeToBp,
				RewardAmount: distributeRewardStr,
				RewardRate: rewardRate,
				ColdStartRewardRate: RewardRate,
				Bp: data.BlockProducer,
				Voter: data.BlockProducer,
				Time: distributeTime.Unix(),
				Status: types.ProcessingStatusDefault,
				TotalBlockReward: totalAmount.String(),
				SingleBlockReward: singleBlkReward.String(),
				SingleColdStartReward: singleBlkColdStartReward.String(),
				TotalVoterVest: totalVest.String(),
				CreatedBlockNumber: data.TotalCount,
				DistributeBlockNumber: endBlk,
				AnnualizedRate: calcAnnualizedROI(data.TotalCount, singleBlkReward, RewardRate, totalVest.String()) ,
			}
			bpInfo := &bpRewardInfo{
				rewardRecord: rec,
				VoterList: voterList,
			}
			list = append(list, bpInfo)
			disType := TransferTypePending
			if CheckIsDistributableBp(data.BlockProducer) {
				//not need to distribute cold start reward to distribute bp
				disType = TransferTypeOfficialBp
			}
			err = sv.sendRewardToAccount(rec, bigAmount, disType)
			if err != nil {
				sv.logger.Errorf("startDistributeToBp: fail to send reward to bp:%v, the error is %v , the rec is %+v", data.BlockProducer, err, rec)
			}

		}
	}
    return list,nil

}

func calcTotalRewardOfBpOnOnePeriod(singleBlkReward decimal.Decimal, blkNum uint64) decimal.Decimal {
	bigNum := new(big.Int).SetUint64(blkNum)
	numDecimal := decimal.NewFromBigInt(bigNum, 0)
	amount := numDecimal.Mul(singleBlkReward)
	return amount
}

func (sv *RewardDistributeService) getPeriodRangeBlockTime(start uint64, end uint64) (time.Time, time.Time, error) {
	sBlkLog,err := db.GetBlockLogByNum(start)
	t := time.Now()
	if err != nil {
		return t,t,err
	}
	eBlkLog,err := db.GetBlockLogByNum(end)
	if err != nil {
		return t,t,err
	}
	return sBlkLog.BlockTime, eBlkLog.BlockTime, nil
}

func (sv *RewardDistributeService) sendRewardToAccount(rewardRec *types.BpRewardRecord, rewardAmount *big.Int, disType int) error {
	if rewardRec == nil {
		sv.logger.Errorf("sendRewardToAccount: fail to send reward with empty BpRewardRecord")
		return errors.New("can't send reward with empty BpRewardRecord")
	}
	var (
		signedTx *prototype.SignedTransaction
		err error
		txHash string
		isTransfer  = true
	)
	if rewardAmount.Uint64() == 0 || disType == TransferTypeInvalideVoter || disType == TransferTypeOfficialBp {
		//amount is 0 or voters's vest is less than utils.MinVoterDistributeVest, not needed to transfer
		// not need send reward to official bp
		sv.logger.Infof("sendRewardToAccount: not need transfer,transfer vest amount to %v is %v", rewardRec.Voter, rewardAmount.Uint64())
		rewardRec.Status = types.ProcessingStatusNotNeedTransfer
		isTransfer  = false
	} else if disType ==  TransferTypePending {
        //not distribute reward immediately
        rewardRec.Status = types.ProcessingStatusPending
		isTransfer  = false
	} else {
		senderAcct,senderPrivkey := config.GetMainNetCosSenderInfo()
		txHash,signedTx,err = utils.GenTransferToVestSignedTx(senderAcct, senderPrivkey, rewardRec.Voter, rewardAmount, "")
		if err != nil {
			sv.logger.Errorf("sendRewardToAccount: fail to generate transfer vest to %v tx, the error is %v, the rec is %+v", rewardRec.Voter, err, rewardRec)
			rewardRec.Status = types.ProcessingStatusGenTxFail
			isTransfer = false
			sv.logFailRewardRecord(rewardRec)
		} else {
			rewardRec.TransferHash = txHash
		}
	}

	// insert reward record to db
	err = db.InsertRewardRecord(rewardRec)
	if err != nil {
		sv.logger.Errorf("sendRewardToAccount: fail to insert reward record to db, the error is %v , the rec is %+v", err, rewardRec)
		sv.logFailRewardRecord(rewardRec)
		return err
	} else if isTransfer{
		//transfer to vest for reward
		_,err = utils.TransferVestBySignedTx(signedTx)
		if err != nil {
			sv.logger.Errorf("sendRewardToAccount: fail to transfer vest to %v for bp:%v, the error is %v, the rec is %+v", rewardRec.Voter, rewardRec.Bp, err, rewardRec)
			sv.logFailRewardRecord(rewardRec)
			return err
		}
	}
    return nil
}

func (sv *RewardDistributeService)logFailRewardRecord(rec *types.BpRewardRecord)  {
	sv.logger.Errorf("startDistribute: the fail reward record is: %+v", rec)

}

func (sv *RewardDistributeService)logDistributeBpInfo(period uint64, list []*types.BpBlockStatistics) {
	sv.logger.Infof("startDistributeToBp: bp Statistics info of period %v is following: \n", period)
	for _,data := range list {
		sv.logger.Infof("%+v \n", data)
	}
}

func GetColdStartRewardByYear(y int) uint64 {
	if val,ok := coldStartRewardMap[y]; ok {
		return val
	}
	return 0
}

func GetEcologicalRewradByYear(y int) uint64 {
    if val,ok := ecologicalRewardMap[y]; ok {
		return val
	}
	return 0
}

func GetTotalRewardByYear(y int) uint64 {
	return  GetColdStartRewardByYear(y) + GetEcologicalRewradByYear(y)
}

// get total reward(cold start + ecological reward) of a block
func CalcSingleBlockRewardByPeriod(period uint64) decimal.Decimal {
	curYear := getYearByPeriod(period)
	yReward := GetTotalRewardByYear(curYear)
	bigAmount := new(big.Int).SetUint64(yReward)
	totalReward := decimal.NewFromBigInt(bigAmount, 0)
	bigYear := new(big.Int).SetUint64(uint64(YearBlkNum))
	singleReward,_ := totalReward.QuoRem(decimal.NewFromBigInt(bigYear, 0), 6)
	return singleReward

}

// get cold start reward of single block by period
func CalcSingleBlockRewardOfColdStartByPeriod(period uint64) *decimal.Decimal {
	curYear := getYearByPeriod(period)
	yReward := GetColdStartRewardByYear(curYear)
	if yReward > 0 {
		bigAmount := new(big.Int).SetUint64(yReward)
		totalReward := decimal.NewFromBigInt(bigAmount, 0)
		bigYear := new(big.Int).SetUint64(uint64(YearBlkNum))
		singleReward,_ := totalReward.QuoRem(decimal.NewFromBigInt(bigYear, 0), 6)
		return &singleReward
	}
	return nil
}


func getYearByPeriod(period uint64) int {
	total := config.ServiceStarPeriodBlockNum + period * config.DistributeInterval
	year := uint64(YearBlkNum)
	curYear := (total / year) + 1
	return int(curYear)
}

// get single block reward of cold start reward
func GetSingleBlockRewardOfColdStart(t time.Time) (*decimal.Decimal,int64) {
	timestamp := t.Unix()
	diff := timestamp - MainNetStartTimeStamp
	year := YearDay * 86400
	curYear := (diff / (int64(year))) + 1
	totalReward := GetColdStartRewardByYear(int(curYear))
	if totalReward > 0 {
		bigReward := new(big.Int).SetUint64(totalReward)
		totalDecimal := decimal.NewFromBigInt(bigReward, 0)
		bigYear := new(big.Int).SetUint64(uint64(year))
		singleReward,_ := totalDecimal.QuoRem(decimal.NewFromBigInt(bigYear, 0), 6)
		return &singleReward,curYear
	}
	return nil, curYear
}


func getTotalVestOfVoters(list []*types.AccountInfo, isFilterDisBp bool) decimal.Decimal {
	total := decimal.New(0, 0)
	for _,acct := range list {
		isContain := true
		if !checkIsValidVoterVest(acct.Vest) {
			isContain = false
		} else if isFilterDisBp && CheckIsDistributableBp(acct.Name) {
			isContain = false
		}
		if isContain {
			bigVal := new(big.Int).SetUint64(acct.Vest)
			total = total.Add(decimal.NewFromBigInt(bigVal, 0))
		}
	}
	total,_ = total.QuoRem(decimal.NewFromFloat(constants.COSTokenDecimals), 6)
	return total
}

func checkIsValidVoterVest(val uint64) bool {
	if val < utils.MinVoterDistributeVest {
		return false
	}
	return true
}

func calcActualReward(distributeReward decimal.Decimal, voterVest decimal.Decimal, totalVest decimal.Decimal) decimal.Decimal {
	if totalVest.Cmp(decimal.New(0, 0)) <= 0 {
		return decimal.New(0, 0)
	}
	reward,_ := distributeReward.Mul(voterVest).QuoRem(totalVest, 6)
	return reward
}

func GetPeriodByBlockNum(blkNum uint64) uint64 {
	if blkNum >= config.ServiceStarPeriodBlockNum {
		blkNum -= config.ServiceStarPeriodBlockNum
	} else {
		return 0
	}
	period := blkNum / config.DistributeInterval
	return period
}

func getStartBlockNumByPeriod(period uint64) uint64 {
	return  config.ServiceStarPeriodBlockNum + config.DistributeInterval*(period-1)
}

func GetBpRewardHistory(period int) ([]*types.RewardInfo, error, int) {
	logger := logs.GetLogger()
	logger.Infof("GetBpRewardHistory: get reward history of past %v period", period)
	curPeriod,err := db.GetLatestDistributedPeriod(true)
	if err != nil {
		logger.Errorf("GetBpRewardHistory: fail to get the latest max distributed record, the error is %v", err)
		return nil, errors.New("fail to get the latest max distributed record"), types.StatusGetLatestPeriodError
	}
	minPeriod,err := db.GetLatestDistributedPeriod(false)
	if err != nil {
		logger.Errorf("GetBpRewardHistory: fail to get the latest min distributed record, the error is %v", err)
		return nil, errors.New("fail to get the latest min distributed record"), types.StatusGetLatestPeriodError
	}
	var sPeriod uint64 = 1
	if uint64(period) < curPeriod {
		sPeriod = curPeriod - uint64(period) + 1
	}
	if sPeriod < minPeriod {
		sPeriod = minPeriod
	}
	logger.Infof("GetBpRewardHistory: start period is %v, current period is %v", sPeriod, curPeriod)
	var infoList []*types.RewardInfo
	for i := sPeriod; i <= curPeriod; i++ {
		rewardInfo := &types.RewardInfo{
			Period: int(i),
		}
		var recList []*types.RewardRecord
		bpRewardList,err := db.GetAllBpRewardHistoryByPeriod(i)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Fail to get reward history of period %v", i)), types.StatusDbQueryError
		}
		if len(bpRewardList) < 1 {
			logger.Errorf("GetBpRewardHistory: the bp reward history of period %v is empty", period)
		} else {
			periodBlkInfo,err := getBlockInfoOfOnePeriodByBpRewardRecord(bpRewardList[0])
			if err != nil {
				return nil, err, types.StatusGetBlockLogError
			}
			rewardInfo.StartBlockNumber = strconv.FormatUint(periodBlkInfo.StartBlockNum, 10)
			rewardInfo.StartBlockTime = strconv.FormatInt(periodBlkInfo.StartBlockTime, 10)
			rewardInfo.EndBlockNumber = strconv.FormatUint(periodBlkInfo.EndBlockNum, 10)
			rewardInfo.EndBlockTime =  strconv.FormatInt(periodBlkInfo.EndBlockTime, 10)
			rewardInfo.DistributeTime = strconv.FormatInt(periodBlkInfo.DistributeTime, 10)
			for _,rec := range bpRewardList {
				singleReward,err := decimal.NewFromString(rec.SingleBlockReward)
				if err != nil {
					logger.Errorf("GetBpRewardHistory: fail to convert single block reward:%v to decimal, the error is %v", rec.SingleBlockReward, err)
					return nil,err, types.StatusConvertRewardError
				}
				annualizedInfo := getAnnualizedInfoByRewardRec(rec)
				record := &types.RewardRecord {
					IsDistributable: CheckIsDistributableBp(rec.Bp),
					BpName: rec.Bp,
					GenBlockCount: strconv.FormatUint(rec.CreatedBlockNumber, 10),
					TotalReward: annualizedInfo.TotalReward,
					VotersVest: rec.TotalVoterVest,

				}
				if CheckIsDistributableBp(rec.Bp) {
					record.RewardRate = utils.FormatFloatValue(RewardRate, 2)
					record.EveryThousandRewards = calcEveryThousandReward(rec.CreatedBlockNumber, singleReward, RewardRate, rec.TotalVoterVest).String()
					record.AnnualizedROI = utils.FormatFloatValue(calcAnnualizedROI(rec.CreatedBlockNumber, singleReward, RewardRate,rec.TotalVoterVest), 6)
				}
				recList = append(recList, record)
			}
		}
		rewardInfo.List = recList
		sort.Sort(rewardInfo)
		infoList = append(infoList, rewardInfo)
	}
    return infoList,nil,types.StatusSuccess

}

func EstimateCurrentPeriodReward() (*types.EstimatedRewardInfoModel, error, int) {
	info := CacheServiceInstance().GetEstimatedRewardInfo()
	if info != nil {
		return info, nil, types.StatusSuccess
	}
	info,err,code := estimateCurrentPeriodReward()
	if err == nil {
		CacheServiceInstance().SetEstimatedRewardInfo(info)
	}
	return info,err,code
}

func estimateCurrentPeriodReward() (*types.EstimatedRewardInfoModel, error, int) {
	logger := logs.GetLogger()
	if isEstimating {
		logger.Errorf("last estimate is not finish")
		return nil, errors.New("last estimate not finish"), types.StatusCalcCreatedBlockError
	}
	isEstimating = true
	defer func() {
		isEstimating = false
	}()
	curTime := time.Now().Unix()
	logger.Infof("EstimateCurrentPeriodReward: estimate current period reward on time: %v", curTime)
	lib,err := db.GetLib()
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to get latest lib on time %v, the error is %v", curTime, err)
		return nil, errors.New("fail to get latest lib"), types.StatusGetLibError
	}
	latestPeriod,err := db.GetLatestDistributedPeriod(true)
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to get latest distribute period on time %v, the error is %v", curTime, err)
		return nil, errors.New("fail to get latest period"), types.StatusGetLatestPeriodError
	}
	eBlkNum := lib
	curPeriod := GetPeriodByBlockNum(lib)
	nextPeriod := curPeriod + 1
	sBlkNum := getStartBlockNumByPeriod(nextPeriod)
	diffBlk := lib - sBlkNum
	if lib < config.ServiceStarPeriodBlockNum {
		if config.ServiceStarPeriodBlockNum > config.DistributeInterval {
			sBlkNum = config.ServiceStarPeriodBlockNum - config.DistributeInterval
			diffBlk = lib - sBlkNum
		}
	} else if latestPeriod + 1 < nextPeriod {
		curPeriod = latestPeriod
		nextPeriod = curPeriod + 1
		sBlkNum = getStartBlockNumByPeriod(nextPeriod)
		eBlkNum = sBlkNum + config.DistributeInterval
		diffBlk = config.DistributeInterval
	}

	logger.Infof("EstimateCurrentPeriodReward: estimate bp reward of next period %v , start block number is %v, end block number is %v, diff block number is %v", nextPeriod, sBlkNum, eBlkNum, eBlkNum-sBlkNum)
    // get all bp on chain
	allBpList,err := db.GetAllBpFromChain()
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to get all bp, the error is %v", err)
	}

	blkSta,err := db.CalcBpGeneratedBlocksOnOnePeriod(sBlkNum, eBlkNum)
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to calculate bp's generated block on time %v, the error is %v", curTime, err)
		return nil,errors.New("fail to calculate bp's generated block"),types.StatusCalcCreatedBlockError
	}

	sBlockLog,err := db.GetBlockLogByNum(sBlkNum)
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to get block log of start block %v, the error is %v", sBlkNum, err)
		return nil,errors.New("fail to get block log of start block"),types.StatusGetBlockLogError
	}
	eBlockLog,err := db.GetBlockLogByNum(eBlkNum)
	if err != nil {
		logger.Errorf("EstimateCurrentPeriodReward: fail to get block log of end block %v, the error is %v", eBlkNum, err)
		return nil,errors.New("fail to get block log of start block"),types.StatusGetBlockLogError
	}
	sBlkTime := sBlockLog.BlockTime.Unix()
	estimateEndBlkTime := sBlkTime + (int64(config.DistributeInterval))
	eBlkTime := eBlockLog.BlockTime.Unix()
	singleBlkReward := CalcSingleBlockRewardByPeriod(nextPeriod)
	logger.Infof("EstimateCurrentPeriodReward: estimate bp reward of next period %v , start block time is %v, end block time is %v", nextPeriod, sBlkTime, eBlkTime)


	var (
		list []*types.EstimatedRewardInfo
	)
	rewardInfo := &types.EstimatedRewardInfoModel{
		StartBlockNumber: strconv.FormatUint(sBlkNum, 10),
		EndBlockNumber: strconv.FormatUint(sBlkNum+config.DistributeInterval, 10),
		DistributeTime: strconv.FormatInt(estimateEndBlkTime, 10),
		UpdateTime: strconv.FormatInt(curTime, 10),
		List: make([]*types.EstimatedRewardInfo, 0),
	}

	notGenBlkBpList := filterNotGeneratedBlockBp(allBpList, blkSta)
	if len(notGenBlkBpList) > 0 {
		blkSta = append(blkSta, notGenBlkBpList...)
	}
	for _,data := range blkSta {
		info := &types.EstimatedRewardInfo{
			BpName: data.BlockProducer,
			GenBlockCount: strconv.FormatUint(data.TotalCount, 10),
			IsDistributable: false,
		}
		totalAmount := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, data.TotalCount)
		info.AccumulatedReward = totalAmount.String()
		logger.Infof("EstimateCurrentPeriodReward: total block reward of bp:%v is %v", data.BlockProducer, totalAmount.String())
		voterList,err := db.GetAllRewardedVotersOfPeriodByBp(data.BlockProducer, sBlkTime, eBlkTime, sBlkNum, eBlkNum)
		if err != nil {
			logger.Errorf("EstimateCurrentPeriodReward: Fail to get all voters who can get reward from bp:%v, the error is %v", data.BlockProducer, err)
			return nil, errors.New("fail to calculate voter's total vest"), types.StatusGetAllVoterVestError
		}
		logger.Infof("bp:%v's valid voter count is %v", data.BlockProducer, len(voterList))
		//not need to filter distribute bp
		totalVoterVest := getTotalVestOfVoters(voterList, false)


		info.EstimatedVotersVest = totalVoterVest.String()
		isDistributable := CheckIsDistributableBp(data.BlockProducer)
		rewardRate := RewardRate
		if !CheckIsDistributableBp(data.BlockProducer) {
			rewardRate = 1
		}

		//calculate total block number generated by this bp
		//estimated block number = generatedBlock / diff * config.DistributeInterval
		estimatedBlkNum,_ := decimal.New(int64(data.TotalCount), 0).Mul(decimal.New(int64(config.DistributeInterval), 0)).QuoRem(decimal.New(int64(diffBlk), 0), 0)
		bigEstimateNum := new(big.Int)
		bigEstimateNum.SetString(estimatedBlkNum.String(), 10)
		estimatedReward := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, bigEstimateNum.Uint64())
		info.EstimatedTotalReward = estimatedReward.String()
		ROI := calcAnnualizedROI(bigEstimateNum.Uint64(), singleBlkReward, RewardRate, info.EstimatedVotersVest)

		if isDistributable {
			info.RewardRate = utils.FormatFloatValue(rewardRate, 2)
			info.IsDistributable = true
			info.EstimatedAnnualizedROI = utils.FormatFloatValue(ROI, 6)
			info.EstimatedThousandRewards = calcEveryThousandReward(bigEstimateNum.Uint64(), singleBlkReward, RewardRate, info.EstimatedVotersVest).String()
		}

		list = append(list, info)

	}

	rewardInfo.List = list
	sort.Sort(rewardInfo)
	logger.Infof("EstimateCurrentPeriodReward: finish estimate current period:%v", curPeriod)
	return rewardInfo, nil, types.StatusSuccess
}

func filterNotGeneratedBlockBp(allBpList []*plugins.ProducerVoteState, genList []*types.BpBlockStatistics) ([]*types.BpBlockStatistics){
	var list []*types.BpBlockStatistics
	for _,voteState := range allBpList {
		isContain := false
		for _,blkStatistics := range genList {
			if voteState.Producer == blkStatistics.BlockProducer {
				isContain = true
				break
			}
		}
		if !isContain {
			info := &types.BpBlockStatistics{
				TotalCount: 0,
				BlockProducer: voteState.Producer,
			}
			list = append(list, info)
		}
	}
	return list
}

func GetHistoricalVotingData() (*types.HistoricalVotingData, error, int) {
	//1. get total voters count
	totalNum,err := db.CalcTotalVotersNumber()
	if err != nil {
		return nil, errors.New("fail to calculate total voters number"), types.StatusGetTotalVoterCountError
	}

	//2. get max ROI
	ROI,err := db.GetMaxROIOfBpReward()
	if err != nil {
		return nil, errors.New("fail to fetch max ROI"), types.StatusGetMaxROIError
	}
	data := &types.HistoricalVotingData{
		VotersNumber: strconv.FormatUint(totalNum, 10),
		MaxROI: utils.FormatFloatValue(ROI, 6),
	}
	return data,nil,types.StatusSuccess

}

func getAnnualizedInfoByRewardRec(rec *types.BpRewardRecord) *types.AnnualizedInfo {
	info := &types.AnnualizedInfo{
		IsDistributable: false,
	}
	//single block reward contain cold start and ecological reward
	singleBlkReward := CalcSingleBlockRewardByPeriod(rec.Period)
	for _,bp := range config.OfficialBpList {
		totalBlkNum := rec.CreatedBlockNumber
		totalReward :=  calcTotalRewardOfBpOnOnePeriod(singleBlkReward, totalBlkNum)
		thousandReward := calcEveryThousandReward(totalBlkNum, singleBlkReward,  rec.RewardRate, rec.TotalVoterVest)
		info.TotalReward = totalReward.String()
		info.RewardRate =  utils.FormatFloatValue(rec.RewardRate, 2)
		info.EveryThousandRewards = thousandReward.String()
		if bp == rec.Bp {
			ROI := calcAnnualizedROI(totalBlkNum, singleBlkReward,  rec.RewardRate, rec.TotalVoterVest)
			info.IsDistributable = true
			info.AnnualizedROI = utils.FormatFloatValue(ROI, 6)
		}
	}
	return info
}

//calculate Every Thousand VEST Reward
func calcEveryThousandReward(totalBlkNum uint64, singleBlkReward decimal.Decimal, rewardRate float64, totalVest string) decimal.Decimal{
	//calculate total reward
	totalReward :=  calcTotalRewardOfBpOnOnePeriod(singleBlkReward, totalBlkNum)
    //EveryThousandReward = totalReward * rate * 1000 / totalVest
    vestDecimal,_ := decimal.NewFromString(totalVest)
	if vestDecimal.Cmp(decimal.New(0, 0)) <= 0 {
		return decimal.New(0, 0)
	}
    rateDecimal := decimal.NewFromFloat(rewardRate)

	thousandReward,_ := totalReward.Mul(rateDecimal).Mul(decimal.New(1000, 0)).QuoRem(vestDecimal, 6)
    return thousandReward
}

//calculate Annualized ROI
func calcAnnualizedROI(totalBlkNum uint64, singleBlkReward decimal.Decimal, rewardRate float64, totalVest string) float64 {
    // ROI = totalReward * rate * 86400 * 365 / totalVest
	vestDecimal,_ := decimal.NewFromString(totalVest)
	if vestDecimal.Cmp(decimal.New(0, 0)) <= 0 {
		return 1.0
	}
	totalReward :=  calcTotalRewardOfBpOnOnePeriod(singleBlkReward, totalBlkNum)
	rateDecimal := decimal.NewFromFloat(rewardRate)
	//single bp generated number = 86400 * 365/21
	ROI,_ := totalReward.Mul(rateDecimal).Mul(decimal.New(YearDay, 0)).QuoRem(vestDecimal.Mul(decimal.New(7, 0)), 6)
	fROI,_ := ROI.Float64()
	return fROI
}

func getBlockInfoOfOnePeriodByBpRewardRecord(rec *types.BpRewardRecord) (*types.OnePeriodBlockInfo,error) {
	logger := logs.GetLogger()
	if rec == nil {
		return nil, errors.New("bp reward record is empty")
	}
	eBlkNum := rec.DistributeBlockNumber
	sBlkNum := eBlkNum - config.DistributeInterval
	eBlkInfo,err := db.GetBlockLogByNum(eBlkNum)
	if err != nil {
		logger.Errorf("GetBpRewardHistory: fail to get block log info of block %v", eBlkNum)
		return nil, errors.New(fmt.Sprintf("fail to get block log info of block %v", eBlkNum))
	}
	sBlkInfo,err := db.GetBlockLogByNum(sBlkNum)
	if err != nil {
		logger.Errorf("GetBpRewardHistory: fail to get block log info of block %v", sBlkNum)
		return nil, errors.New(fmt.Sprintf("fail to get block log info of block %v", sBlkNum))
	}
	info := &types.OnePeriodBlockInfo{
		StartBlockNum: sBlkNum,
		StartBlockTime: sBlkInfo.BlockTime.Unix(),
		EndBlockNum: eBlkNum,
		EndBlockTime: eBlkInfo.BlockTime.Unix(),
		DistributeTime: rec.Time,
	}
	return info, nil
}

func CheckIsDistributableBp(bpName string) bool {
	for _,bp := range config.OfficialBpList {
		if bp == bpName {
			return true
		}
	}
	return false
}

