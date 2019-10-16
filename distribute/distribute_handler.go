package distribute

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"errors"
	"fmt"
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
    RewardRate float64 = 0.8
    DistributeInterval = 7 * 86400 // 7 * 86400 block
	timeInterval = 1 * time.Minute
	ServiceStarPeriodBlockNum uint64 = 0
    //cold start reward
	ColdStartRewardMaxYear = 5
	//Ecological reward
	EcologicalRewardMaxYear = 12
	YearBlkNum = YearDay * 86400
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

func NewDistributeService() *RewardDistributeService {
	return &RewardDistributeService{
		logger: logs.GetLogger(),
	}
}

func (sv* RewardDistributeService)StartDistributeService() error {
	ticker := time.NewTicker(time.Duration(timeInterval))
	go func() {
		for {
			select {
			case <- ticker.C:
				sv.handleDistribute()
			case <- sv.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
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

	fmt.Printf("handleDistribute:isHandling status is %v \n", sv.isHandling)
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
	fmt.Printf("handleDistribute: new period is %v,lib is %v \n", curPeriod, lib)
    if curPeriod < 1 {
		sv.isHandling = false
		return
	} else {
		sBlkNum := (curPeriod - 1) * DistributeInterval
		eBlkNum := curPeriod * DistributeInterval
		//judge is need to distribute for this period
		latestPeriod,err := db.GetLatestDistributedPeriod(true)
		fmt.Printf("handleDistribute: latestPeriod is %v \n", latestPeriod)
		if err != nil {
			sv.logger.Errorf("handleDistribute: fail to get latest distributed period, the error is %v", err)
			sv.isHandling = false
			return
		}

		if latestPeriod + 1 < curPeriod {
			// last period has not distribute
			// distribute last period reward
			lastPeriod := curPeriod - 1
			s := (lastPeriod-1) * DistributeInterval
			e := lastPeriod * DistributeInterval
			sv.logger.Infof("handleDistribute: distribute reward of last period:%v, current period is %v", lastPeriod, curPeriod)
			lastTimeStamp := t - int64(lib - s)
			lastTime := time.Unix(lastTimeStamp, 0)
			sv.startDistribute(lastPeriod, lastTime ,s, e)
			latestPeriod = curPeriod - 1
		}

		if latestPeriod + 1 == curPeriod {
			//need distribute this period reward
			sv.startDistribute(curPeriod, curTime, sBlkNum, eBlkNum)
		}
	}
	sv.isHandling = false
	fmt.Printf("endDistribute:isHandling status is %v \n", sv.isHandling)
}

func (sv *RewardDistributeService) startDistribute(period uint64, t time.Time, startBlk uint64, endBlk uint64)  {
	sv.logger.Infof("Start this round reward distribute of period %v", period)

	curTime := t.Unix()
	lastPeriod := curTime - (int64(DistributeInterval))
	sv.logger.Infof("startDistribute: start distribute of this week,the block number range is from:%v to:%v", startBlk, endBlk)
	//calculate single block reward of this year
	singleReward,curYear := GetSingleBlockRewardOfColdStart(t)
	if singleReward == nil {
		sv.logger.Errorf("startDistribute: not support distribute for year %v", curYear)
		return
	}
	//1. distribute reward to bp
	bpSingleReward := CalcSingleBlockRewardByPeriod(period)
	bpRewardInfoList,_ := sv.startDistributeToBp(period, t, bpSingleReward, startBlk, endBlk)
	//2. distribute reward to all voters
	for _,bp := range OfficialBpList {
		sv.logger.Infof("startDistribute: single block reward of year:%v is %v", curYear, singleReward.String())
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
			voterList,err = db.GetAllRewardedVotersOfPeriodByBp(bp, lastPeriod, curTime, startBlk, endBlk)
			if err != nil {
				sv.logger.Errorf("startDistribute: Fail to get all voters who can get reward from bp:%v, the error is %v", bp, err)
				continue
			} else {
				//calculate all voter's total vest
				totalVest = getTotalVestOfVoters(voterList)
				sv.logger.Infof("startDistribute: total vest of all voters of bp:%v is %v", bp, totalVest.String())
			}
		}
		sv.logger.Infof("startDistribute: this round voters number of bp:%v is %v", bp, len(voterList))

		if isBlkCntValid {
				sv.logger.Infof("startDistribute: generated block of bp:%v is %v", bp, generatedBlkNum)
				//calculate total block reward of this bp
				totalReward := calcTotalRewardOfBpOnOnePeriod(*singleReward, generatedBlkNum)
				sv.logger.Infof("startDistribute: total block reward of bp:%v is %v", bp, totalReward.String())
				//calculate actually distributed rewards (total reward * rate)
				distributeReward := totalReward.Mul(decimal.NewFromFloat(RewardRate))
				sv.logger.Infof("startDistribute: total distribute block reward of bp:%v is %v", bp, distributeReward.String())
				for _,acct := range voterList {
					bigVal := new(big.Int).SetUint64(acct.Vest)
					vest := decimal.NewFromBigInt(bigVal, 0)
					actualReward :=  calcActualReward(distributeReward, vest, totalVest)
					distributeReward := actualReward
					transferReward := new(big.Int)
					transferReward.SetString(distributeReward.String(), 10)
					distributeReward,_ = distributeReward.QuoRem(decimal.NewFromFloat(constants.COSTokenDecimals), 6)
					sv.logger.Infof("startDistribute: calculate voter:%v's reward is %v, actually reward is %v", acct.Name, distributeReward, transferReward)

					//create reward record
					rewardRec := &types.BpRewardRecord{
						Id: utils.GenerateId(t, acct.Name, bp, "reward to voter"),
						Period: period,
						RewardType: types.RewardTypeToVoter,
						Amount: distributeReward.String(),
						RewardRate: RewardRate,
						Bp: bp,
						Voter: acct.Name,
						Time: curTime,
						Status: types.ProcessingStatusDefault,
						Vest: utils.CalcActualVest(acct.Vest),
						TotalBlockReward: totalReward.String(),
						SingleBlockReward: singleReward.String(),
						TotalVoterVest: totalVest.String(),
						CreatedBlockNumber: generatedBlkNum,
						DistributeBlockNumber: endBlk,
						AnnualizedRate: calcAnnualizedROI(generatedBlkNum, *singleReward, RewardRate, totalVest.String()),
					}
					sv.sendRewardToAccount(rewardRec, transferReward)
				}
		}

	}
	sv.logger.Infof("startDistribute: finish this round distribute of period:%v", period)
}


//
// distribute reward to all bp that have generated blocks successfully
//
func (sv *RewardDistributeService) startDistributeToBp(period uint64,t time.Time, singleBlkReward decimal.Decimal, startBlk uint64, endBlk uint64) ([]*bpRewardInfo, error){
	sv.logger.Infof("startDistributeToBp: start distribute reward to bp on block range(from:%v,to:%v)", startBlk, endBlk)
	//1. calculate all bp's total generated blocks
	statistics,err := db.CalcBpGeneratedBlocksOnOnePeriod(startBlk, endBlk)
	if err != nil {
		sv.logger.Errorf("startDistributeToBp: fail to calculate bp block statistics on period:%v , the error is %v", period ,err)
		return nil, err
	}
	count := len(statistics)
	eTime := t.Unix()
	sTime := eTime - (int64(DistributeInterval))
	sv.logger.Infof("startDistributeToBp: start time is %v, end time is %v", sTime, eTime)
	var list []*bpRewardInfo
	if count < 1 {
		sv.logger.Errorf("startDistributeToBp: block statistics is empty on period:%v", period)
	} else {
		sv.logDistributeBpInfo(period, statistics)
		for _,data := range statistics {
			// calculate total reward of bp
			totalAmount := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, data.TotalCount)
			sv.logger.Infof("startDistributeToBp: total block reward of bp:%v is %v", data.BlockProducer, totalAmount.String())
			// Multiply COSTokenDecimals
			distributeReward := totalAmount.Mul(decimal.NewFromFloat(constants.COSTokenDecimals))
			bigAmount := new(big.Int)
			bigAmount.SetString(distributeReward.String(), 10)
			voterList,err := db.GetAllRewardedVotersOfPeriodByBp(data.BlockProducer, sTime, eTime, startBlk, endBlk)
			totalVest := decimal.New(-1, 0)
			if err != nil {
				sv.logger.Errorf("startDistributeToBp: fail to get all voters of bp:%v on this period, the error is %v", data.BlockProducer, err)
			} else {
				totalVest = getTotalVestOfVoters(voterList)
			}
			sv.logger.Infof("startDistributeToBp: bp:%v's totalVest is %v,voters count is %v", data.BlockProducer, totalVest.String(), len(voterList))
			//send reward to bp
			rec := &types.BpRewardRecord{
				Id: utils.GenerateId(t, data.BlockProducer, data.BlockProducer, "Reward to bp"),
				Period: period,
				RewardType: types.RewardTypeToBp,
				Amount: totalAmount.String(),
				RewardRate: 1,
				Bp: data.BlockProducer,
				Voter: data.BlockProducer,
				Time: t.Unix(),
				Status: types.ProcessingStatusDefault,
				TotalBlockReward: totalAmount.String(),
				SingleBlockReward: singleBlkReward.String(),
				TotalVoterVest: totalVest.String(),
				CreatedBlockNumber: data.TotalCount,
				DistributeBlockNumber: endBlk,
				AnnualizedRate: calcAnnualizedROI(data.TotalCount, singleBlkReward, RewardRate, totalVest.String()) ,
			}
			bpInfo := &bpRewardInfo{
				rewardRecord: rec,
				VoterList: voterList,
			}
			fmt.Printf("voters count of %v is %v \n", data.BlockProducer, len(voterList))
			list = append(list, bpInfo)
			err = sv.sendRewardToAccount(rec, bigAmount)
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

func (sv *RewardDistributeService) sendRewardToAccount(rewardRec *types.BpRewardRecord, rewardAmount *big.Int) error {
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
	if rewardAmount.Uint64() == 0 {
		//amount is 0, not needed to transfer
		sv.logger.Infof("sendRewardToAccount: transfer vest amount to %v is 0", rewardRec.Voter)
		rewardRec.Status = types.ProcessingStatusNotNeedTransfer
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

func getYearByPeriod(period uint64) int {
	total := ServiceStarPeriodBlockNum + period * DistributeInterval
	year := uint64(YearDay) * 86400
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

func getTotalVestOfVoters(list []*types.AccountInfo) decimal.Decimal {
	total := decimal.New(0, 0)
	for _,acct := range list {
		bigVal := new(big.Int).SetUint64(acct.Vest)
		total = total.Add(decimal.NewFromBigInt(bigVal, 0))
	}
	total,_ = total.QuoRem(decimal.NewFromFloat(constants.COSTokenDecimals), 6)
	return total
}

func calcActualReward(distributeReward decimal.Decimal, voterVest decimal.Decimal, totalVest decimal.Decimal) decimal.Decimal {
	if totalVest.Cmp(decimal.New(0, 0)) <= 0 {
		return decimal.New(0, 0)
	}
	reward,_ := distributeReward.Mul(voterVest).QuoRem(totalVest, 6)
	return reward
}

func GetPeriodByBlockNum(blkNum uint64) uint64 {
	if blkNum > ServiceStarPeriodBlockNum {
		blkNum -= ServiceStarPeriodBlockNum
	}
	period := blkNum / DistributeInterval
	return period
}

func getStartBlockNumByPeriod(period uint64) uint64 {
	return  ServiceStarPeriodBlockNum + DistributeInterval*(period-1)
}

func GetBpRewardHistory(period int) ([]*types.RewardInfo, error, int) {
	fmt.Printf("GetBpRewardHistory: period is %v", period)
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
	fmt.Printf("start priod is %v, crrent period is %v \n", sPeriod, curPeriod)
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
				annualizedInfo := getAnnualizedInfoByRewardRec(rec)
				rate := utils.FormatFloatValue(float64(rec.RewardRate), 2)
				record := &types.RewardRecord {
					IsDistributable: CheckIsDistributableBp(rec.Bp),
					BpName: rec.Bp,
					GenBlockCount: strconv.FormatUint(rec.CreatedBlockNumber, 10),
					TotalReward: annualizedInfo.TotalReward,
					RewardRate: rate,
					VotersVest: rec.TotalVoterVest,
					EveryThousandRewards: annualizedInfo.EveryThousandRewards,
					AnnualizedROI: annualizedInfo.AnnualizedROI,
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

func EstimateCurrentPeriodReward() ([]*types.EstimatedRewardInfo, error, int) {
	logger := logs.GetLogger()
	curTime := time.Now().Unix()
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
	if latestPeriod + 1 < nextPeriod {
		curPeriod = latestPeriod
		nextPeriod = curPeriod + 1
		sBlkNum = getStartBlockNumByPeriod(nextPeriod)
		eBlkNum = sBlkNum + DistributeInterval
		diffBlk = DistributeInterval
	}

	logger.Infof("EstimateCurrentPeriodReward: estimate bp reward of next period %v , start block number is %v, end block number is %v, diff block number is %v", nextPeriod, sBlkNum, eBlkNum, eBlkNum-sBlkNum)
	blkSta,err := db.CalcBpGeneratedBlocksOnOnePeriod(sBlkNum, eBlkNum)
	if err != nil {
		return nil,errors.New("fail to calculate bp's generated block"),types.StatusCalcCreatedBlockError
	}
	singleBlkReward := CalcSingleBlockRewardByPeriod(nextPeriod)

	sTime := curTime - int64(diffBlk)
	var list []*types.EstimatedRewardInfo
	for _,data := range blkSta {
		info := &types.EstimatedRewardInfo{
			BpName: data.BlockProducer,
			GenBlockCount: strconv.FormatUint(data.TotalCount, 10),
			IsDistributable: false,
		}
		totalAmount := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, data.TotalCount)
		info.AccumulatedReward = totalAmount.String()
		logger.Infof("EstimateCurrentPeriodReward: total block reward of bp:%v is %v", data.BlockProducer, totalAmount.String())
		voterList,err := db.GetAllRewardedVotersOfPeriodByBp(data.BlockProducer, sTime, curTime, sBlkNum, eBlkNum)
        if err != nil {
			logger.Errorf("EstimateCurrentPeriodReward: Fail to get all voters who can get reward from bp:%v, the error is %v", data.BlockProducer, err)
			return nil, errors.New("fail to calculate voter's total vest"), types.StatusGetAllVoterVestError
		}
		totalVoterVest := getTotalVestOfVoters(voterList)
		info.EstimatedVotersVest = totalVoterVest.String()
		isDistributable := CheckIsDistributableBp(data.BlockProducer)
		ROI := calcAnnualizedROI(data.TotalCount, singleBlkReward, RewardRate, info.EstimatedVotersVest)
		if isDistributable {
			info.RewardRate = utils.FormatFloatValue(RewardRate, 2)
			info.IsDistributable = true
			info.EstimatedAnnualizedROI = utils.FormatFloatValue(ROI, 6)
			info.EstimatedThousandRewards = calcEveryThousandReward(data.TotalCount, singleBlkReward, RewardRate, info.EstimatedVotersVest).String()
		}
		//calculate total block number generated by this bp
		//estimated block number = generatedBlock / diff * DistributeInterval
		estimatedBlkNum,_ := decimal.New(int64(data.TotalCount), 0).Mul(decimal.New(int64(DistributeInterval), 0)).QuoRem(decimal.New(int64(diffBlk), 0), 0)
		fmt.Printf("estimatedBlkNum is %v \n", estimatedBlkNum.String())
		bigEstimateNum := new(big.Int)
		bigEstimateNum.SetString(estimatedBlkNum.String(), 10)
		fmt.Printf("bigEstimateNum is %v \n", bigEstimateNum.String())
		estimatedReward := calcTotalRewardOfBpOnOnePeriod(singleBlkReward, bigEstimateNum.Uint64())
		info.EstimatedTotalReward = estimatedReward.String()

		list = append(list, info)
	}
	return list, nil, types.StatusSuccess
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
	for _,bp := range OfficialBpList {
		if bp == rec.Bp {
			totalBlkNum := rec.CreatedBlockNumber
			totalReward :=  calcTotalRewardOfBpOnOnePeriod(singleBlkReward, totalBlkNum)
			thousandReward := calcEveryThousandReward(totalBlkNum, singleBlkReward,  rec.RewardRate, rec.TotalVoterVest)
			ROI := calcAnnualizedROI(totalBlkNum, singleBlkReward,  rec.RewardRate, rec.TotalVoterVest)
			info.IsDistributable = true
			info.RewardRate = utils.FormatFloatValue(RewardRate, 2)
			info.TotalReward = totalReward.String()
			info.EveryThousandRewards = thousandReward.String()
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
	sBlkNum := eBlkNum - DistributeInterval
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
	for _,bp := range OfficialBpList {
		if bp == bpName {
			return true
		}
	}
	return false
}

