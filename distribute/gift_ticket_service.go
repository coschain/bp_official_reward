package distribute

import (
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"github.com/sirupsen/logrus"
	"time"
)


type GiftTicketCheckService struct {
	stopCh     chan bool
	logger  *logrus.Logger
	isHandling bool
}

var (
	giftRewardList []*types.GiftTicketRewardInfo
    ticketCheckSv *GiftTicketCheckService
)


func StartGiftTicketCheckService()  {
	ticker := time.NewTicker(time.Minute * 3)
	ticketCheckSv = &GiftTicketCheckService{
		logger: logs.GetLogger(),
	}
	go func() {
		for {
			select {
			case <- ticker.C:
				ticketCheckSv.checkGiftReward()
			case <- ticketCheckSv.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func CloseGiftTicketCheckService()  {
	if ticketCheckSv != nil {
		ticketCheckSv.stopCh <- true
		close(ticketCheckSv.stopCh)
	}
}

func (sv *GiftTicketCheckService) checkGiftReward()  {
	if sv.isHandling {
		sv.logger.Info("checkGiftReward: last round gift reward check is not finish")
		return
	}
	sv.isHandling = true
	defer func() {
		sv.isHandling = false
	}()
	sv.logger.Info("checkGiftReward: start this round gift reward check")
	sInfo := fetchEstimateStatisticsInfo("checkGiftReward")
	sBlkNum := sInfo.startBlock
	eBlkNum := sInfo.endBlock
	list,err := fetchLatestGiftRewardInfo(sBlkNum, eBlkNum)
	if err != nil {
		sv.logger.Errorf("checkGiftReward: fail to get latest gift reward info,the error is %v", err)
	} else {
		giftRewardList = list
	}

	sv.logger.Info("checkGiftReward: finish this round gift reward check")
}

func fetchLatestGiftRewardInfo(sBlkNum,eBlkNum uint64) ([]*types.GiftTicketRewardInfo,error) {
	list,err := db.GetGiftRewardOfOfficialBpOnRange(sBlkNum, eBlkNum)
	if err != nil {
		return nil,err
	}
	return list,nil
}