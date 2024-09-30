package snapshot

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"time"

	"github.com/sirupsen/logrus"
)

type BpSyncService struct {
	stopCh      chan bool
	logger      *logrus.Logger
}

func NewBpSyncService() (*BpSyncService, error) {
	logger := logs.GetLogger()
	service := &BpSyncService{
		logger: logger,
	}
	return service, nil
}

func (s *BpSyncService) StartSyncService() {
	s.stopCh = make(chan bool)
	ticker := time.NewTicker(time.Duration(config.SnapshotTimeInterval))
	go func() {
		for {
			select {
			case <- ticker.C:
				s.snapshot()
			case <- s.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *BpSyncService) StopSyncService()  {
	s.stopCh <- true
	close(s.stopCh)
}

func (s *BpSyncService) snapshot() {
	curTime := time.Now()
	s.logger.Infof("Start snapshot: the timestamp is %v", curTime.Unix())

	// lib,err := db.GetLib()
	// if err != nil {
	// 	s.logger.Errorf("snapshot: fail to get latest lib on time %v, the error is %v", curTime.Unix(), err)
	// 	return
	// }
	// year := lib / (distribute.YearBlkNum) + 1
	// if year > distribute.ColdStartRewardMaxYear {
	// 	//after 5 year, not need to snapshot
	// 	s.logger.Infof("snapshot: not need to snapshot on time:%v, on year %v", curTime, year)
	// 	return
	// }

	////1. get official bp vote record
	//recList,err := db.GetBpVoteRecords(curTime, config.OfficialBpList)
	//if err != nil {
	//	s.logger.Errorf("snapshot: Fail to get vote records of official bp on time:%v, the error is %v", err, curTime)
	//	s.logger.Infoln("Finish this round snapshot")
	//	//sync official bp's voter info
	//	s.syncVotersAccountInfo(curTime, config.OfficialBpList)
	//	return
	//} else {
	//	if len(recList) > 0 {
	//		// batch insert vote record to db
	//		err := db.BatchInsertVoteRecord(recList)
	//		if err != nil {
	//			s.logger.Errorf("snapshot: Fail to batch insert official bp vote record on time:%v, the error is %v", curTime, err)
	//			s.logger.Infoln("Finish this round snapshot")
	//			//sync official bp's voter info
	//			s.syncVotersAccountInfo(curTime, config.OfficialBpList)
	//			return
	//		}
	//	} else {
	//		s.logger.Infof("snapshot: official bp vote record is empty on time:%v", curTime)
	//	}
	//
	//}

	//2. get bp's vote relation
	rList,err := db.GetBpVoteRelation(curTime, config.OfficialBpList)
	if err != nil {
		s.logger.Errorf("snapshot: Fail to get bp vote relation, the error is %v", err)
	} else {
		// batch insert vote relation to db
		err = db.BatchInsertVoteRelation(rList)
		if err != nil {
			s.logger.Errorf("snapshot: Fail to batch insert vote relations on time:%v, the error is %v", curTime ,err)
		}
	}
	bpList := config.OfficialBpList
	if len(rList) > 0 {
		var list []string
		for _,relation := range rList {
			list = append(list, relation.Producer)
		}
		bpList = list
	}
	//3.get voter vest info and insert to db
	s.syncVotersAccountInfo(curTime , bpList)
	s.logger.Infoln("Finish this round snapshot")
}

func (s *BpSyncService) syncVotersAccountInfo(t time.Time, bpList []string)  {
	voterList,err := db.GetVoterInfoByBp(t, bpList)
	if err != nil {
		s.logger.Errorf("snapshot: Fail to get all voters vest info, the error is %v", err)
	} else {
		//batch insert user vest info to db
		err = db.BatchInsertUserVestInfo(voterList)
		if err != nil {
			s.logger.Errorf("snapshot: Fail to batch insert voters vest info on time:%v, the error is %v", t, err)
		}
	}
}
