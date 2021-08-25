package check

import (
	"bp_official_reward/types"
	"fmt"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/utils"
	"time"
)

const checkInterval = 2*time.Minute

type RewardResultChecker struct {
	isChecking bool
	stopCh      chan bool
}

func NewRewardResultChecker() *RewardResultChecker {
	return &RewardResultChecker{
		isChecking: false,
	}
}

func (checker* RewardResultChecker) StartCheck() {
	checker.stopCh = make(chan bool)
	ticker := time.NewTicker(time.Duration(checkInterval))
	go func() {
		for {
			select {
			case <- ticker.C:
				checker.checkRewardResult()
			case <- checker.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}


func (checker* RewardResultChecker) StopCheck() {
	checker.stopCh <- true
	close(checker.stopCh)
}

func (checker* RewardResultChecker) checkRewardResult()  {
	logger := logs.GetLogger()
	if checker.isChecking {
		logger.Errorf("Last round check not finish")
		return
	}
	logger.Debugln("start new round check reward result")
	checker.isChecking = true
	rewardRecs,err := db.GetAllNotSuccessRewardRecords()
	if err != nil {
		logger.Errorf(fmt.Sprintf("CheckRewardResult: fail to get reward records, the error is %v", err))
		checker.isChecking = false
		return
	}
	logger.Debugf("CheckRewardResult: reward records number is %v", len(rewardRecs))
	for _,reward := range rewardRecs {
		txHash := utils.GetTxHashFromCOSTransferTxId(reward.TransferHash)
		if utils.CheckIsNotEmptyStr(txHash) {
			isSuc := false
			isSuc,err = db.CheckTransferToVestTxIsSuccess(txHash)
			if isSuc {
				//modify to processed
				rId := reward.Id
				err = db.MdRewardProcessStatus(rId, types.ProcessingStatusSuccess)
				if err != nil {
					logger.Errorf(fmt.Sprintf("CheckRewardResult: fail to modify reward record: %v status to true, the error is %v",  rId, err))
				}
			}
		}
	}
	checker.isChecking = false
	logger.Debugln("Finish this round check",)

}