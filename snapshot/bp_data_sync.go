package snapshot

import (
	"bp_official_reward/db"
	"bp_official_reward/distribute"
	"bp_official_reward/logs"
	"github.com/sirupsen/logrus"
	"time"
)

const syncInterval = 3 * time.Hour //every 3 hour to snapshot

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
	ticker := time.NewTicker(time.Duration(syncInterval))
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
	d,year := distribute.GetSingleBlockReward(curTime)
	if d == nil {
		//after 5 year, not need to snapshot
		s.logger.Infof("snapshot: not need to snapshot on time:%v, on year %v", curTime, year)
		return
	}

	//1.get voter vest info and insert to db
	voterList,err := db.GetVoterInfoByBp(curTime, distribute.OfficialBpList)
	if err != nil {
		s.logger.Errorf("snapshot: Fail to get all voters vest info, the error is %v", err)
	} else {
		//batch insert user vest info to db
		err = db.BatchInsertUserVestInfo(voterList)
		if err != nil {
			s.logger.Errorf("snapshot: Fail to batch insert voters vest info on time:%v, the error is %v", curTime, err)
		}
	}


	//2. get official bp vote record
	recList,err := db.GetBpVoteRecords(curTime, distribute.OfficialBpList)
	if err != nil {
		s.logger.Errorf("snapshot: Fail to get vote records of official bp on time:%v, the error is %v", err, curTime)
		s.logger.Infoln("Finish this round snapshot")
		return
	} else {
		if len(recList) > 0 {
			// batch insert vote record to db
			err := db.BatchInsertVoteRecord(recList)
			if err != nil {
				s.logger.Errorf("snapshot: Fail to batch insert official bp vote record on time:%v, the error is %v", curTime, err)
				s.logger.Infoln("Finish this round snapshot")
				return
			}
		} else {
			s.logger.Infof("snapshot: official bp vote record is empty on time:%v", curTime)
		}

	}

	//3. get bp's vote relation
	rList,err := db.GetBpVoteRelation(curTime, distribute.OfficialBpList)
	if err != nil {
		s.logger.Errorf("snapshot: Fail to get bp vote relation, the error is %v", err)
	} else {
		// batch insert vote relation to db
		err = db.BatchInsertVoteRelation(rList)
		if err != nil {
			s.logger.Errorf("snapshot: Fail to batch insert vote relations on time:%v, the error is %v", curTime ,err)
		}
	}
	s.logger.Infoln("Finish this round snapshot")
}
