package db

import (
	"bp_official_reward/config"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coschain/contentos-go/app/plugins"
	"github.com/coschain/contentos-go/iservices"
	"github.com/coschain/contentos-go/prototype"
	"github.com/ethereum/go-ethereum/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"strconv"
	"strings"
	"time"
)

var (
	serDB *gorm.DB
	cosNodeDb *gorm.DB
	cosNodeDbHost string
	curBlockHeight uint64
	checkInterval = 2 * time.Minute
	stop  chan bool
	isChecking bool
	batchInsertLimit = 5000
)

func StartDbService() error {
	logger := logs.GetLogger()
	logger.Debugln("Start db service")
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("StartDbService: fail to get db,the error is %v", err)
		return err
	}
	serDB = db
	//create tables if not exist
	err = createTables(serDB)
	if err != nil {
		return err
	}

	nodeDb,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("StartDbService: fail to get cos observe node db,the error is %v", err)
		return err
	}
	cosNodeDb = nodeDb
	checkCosNodeDbValid()
    return nil
}


func createTables(db *gorm.DB) (err error) {
	if db == nil {
		return errors.New("service db is empty")
	}
	logger := logs.GetLogger()
	//create AccountInfo table
	if !db.HasTable(&types.AccountInfo{}) {
		if err = db.CreateTable(&types.AccountInfo{}).Error; err != nil {
			logger.Errorf("fail to create AccountInfo table,the error is %v",err)
			return err
		}
	}
	//create BpVoteRelation table
	if !db.HasTable(&types.BpVoteRelation{}) {
		if err = db.CreateTable(&types.BpVoteRelation{}).Error; err != nil {
			logger.Errorf("fail to create BpVoteRelation table,the error is %v",err)
			return err
		}
	}

	//create BpVoteRecord table
	if !db.HasTable(&types.BpVoteRecord{}) {
		if err = db.CreateTable(&types.BpVoteRecord{}).Error; err != nil {
			logger.Errorf("fail to create BpVoteRecord table,the error is %v",err)
			return err
		}
	}

	//create BpRewardRecord table
	if !db.HasTable(&types.BpRewardRecord{}) {
		if err = db.CreateTable(&types.BpRewardRecord{}).Error; err != nil {
			logger.Errorf("fail to create BpRewardRecord table,the error is %v",err)
			return err
		}
	}
	return
}

func getCosObserveNodeDb() (*gorm.DB, error) {
	if cosNodeDb != nil {
		return cosNodeDb,nil
	}
	logger := logs.GetLogger()
	list,err := config.GetCosObserveNodeDbConfigList()
	if err != nil {
		logger.Errorf("GetCosObserveNodeDb: fail to get cos observe node db config, the error is %v", err)
		return nil, errors.New("open db: fail to get observe node db config")
	}
	var dbErr error
	for _,cf := range list {
		db,err := openDb(cf)
		if err != nil {
			logger.Errorf("GetCosObserveNodeDb: fail to open db, the error is %v", err)
			dbErr = err
		} else if db != nil {
			cosNodeDbHost = cf.Host
			cosNodeDb = db
			return db,nil
		}
	}
	return nil, dbErr
}


// get database of reward service
func getServiceDB() (*gorm.DB,error){
	log := logs.GetLogger()
	if serDB == nil {
		dbCfg,err := config.GetRewardDbConfig()
		if err != nil {
			log.Errorf("getServiceDB: fail to get db config, the error is %v ", err)
			return nil, errors.New("open db: fail to get service db config")
		}

		db,err := openDb(dbCfg)
		if err != nil {
			log.Errorf("getServiceDB: fail to open db, the error is %v ", err)
			return nil,errors.New("open db: fail to open")
		}
		return db,nil
	}
	return serDB,nil
}

func openDb(dbCfg *config.DbConfig) (*gorm.DB, error) {
	log := logs.GetLogger()
	source := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port,dbCfg.DbName)
	db,err := gorm.Open(dbCfg.Driver, source)
	if err != nil {
		log.Errorf("openDb: fail to open db: %v, the error is %v ", dbCfg, err)
		return nil,errors.New("fail to open db")
	}
	return db,nil
}

// Timing check the database status regularly(check block height change)
func checkCosNodeDbValid()  {
	ticker := time.NewTicker(checkInterval)
	go func() {
		for {
			select {
			case <- ticker.C:
				checkBlockStatus()
			case <- stop:
				ticker.Stop()
			}

		}
	}()
}

func checkBlockStatus()  {
	logger := logs.GetLogger()
	if isChecking {
		logger.Infoln("last round check block status not finish")
		return
	}
	isChecking = true
	defer func() {
		isChecking = false
	}()
	logger.Infoln("start check block status")
	if cosNodeDb != nil {
		var process iservices.DeprecatedBlockLogProgress
		err := cosNodeDb.Take(&process).Error
		if err != nil {
			logger.Errorf("checkBlockStatus: fail to get cos chain block process")
		} else {
			if  process.BlockHeight <= curBlockHeight {
				logger.Infof("new block height is %v, cache block height is %v", process.BlockHeight, curBlockHeight)
				logger.Infof("checkBlockStatus: Need to switch to another cos observe node db")
				//need to switch full node db
				list,err := config.GetCosObserveNodeDbConfigList()
				if err != nil {
					logger.Errorf("checkBlockStatus: fail to get db config list, the error is %v", err)
				} else {
					for _,cf := range list {
						if cf.Host != cosNodeDbHost {
							db,err := openDb(cf)
							if err == nil {
								logger.Infof("checkBlockStatus: success to switch origin cos node db:%v to new db:%v", cosNodeDbHost, cf.Host)
								cosNodeDb = db
								cosNodeDbHost = cf.Host
								break
							} else {
								logger.Errorf("checkBlockStatus: fail to switch new db, the error is %v", err)
							}
						}
					}
				}
			}
			curBlockHeight = process.BlockHeight
		}

	}
	logger.Infoln("finish this round check block status")

}

func CloseDbService() {
	logger := logs.GetLogger()
	logger.Infoln("Close my sql database")
	if serDB != nil {
		if err := serDB.Close(); err != nil {
			logger.Errorf("Fail to close serve db, the error is %v", err)
		}
	}

	if cosNodeDb != nil {
		if err := cosNodeDb.Close(); err != nil {
			logger.Errorf("Fail to close cos observe node db, the error is %v", err)
		}
	}
}

//
// get vest info from cos chain
//
func GetUserVestInfo(acctName string, t time.Time) (*types.AccountInfo, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetUserVestInfo: fail to get observe node db, the error is %v", db)
		return nil, err
	}

	var holder plugins.Holder
	err = db.Take(&holder, "name = ?", acctName).Error
	if err != nil {
		logger.Errorf("GetUserVestInfo: fail to get vest of %v, the error is %v", acctName, err)
		return nil, err
	}
	info := &types.AccountInfo{
		AccountId: utils.GenerateId(t, holder.Name),
		Name: holder.Name,
		Balance: holder.Balance,
		Vest: holder.Vest,
		StakeVestFromMe: holder.StakeVestFromMe,
		Time: t.Unix(),
	}
	return info, nil
}

//
// get all voter's info of all official bpList
//
func GetVoterInfoByBp(t time.Time, bpList []string) ([]*types.AccountInfo, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetVoterInfoByBp: fail to get observe node db, the error is %v", db)
		return nil, err
	}
	filter := "name in (SELECT DISTINCT voter from producer_vote_states WHERE " + getDbFilterCondition(bpList, "producer") + ")"
	var (
	    holders []*plugins.Holder
		voterList []*types.AccountInfo
	)
	err = db.Where(filter).Find(&holders).Order("name ASC").Error
	if err != nil {
		logger.Errorf("GetVoterInfoByBp: fail to get voter info of bp, the error is %v", err)
		return nil, err
	}
	for _,holder := range holders {
		voter := &types.AccountInfo{
			AccountId: utils.GenerateId(t, holder.Name),
			Name: holder.Name,
			Balance: holder.Balance,
			Vest: holder.Vest,
			StakeVestFromMe: holder.StakeVestFromMe,
			Time: t.Unix(),
		}
		voterList = append(voterList, voter)
	}
	return voterList,nil
}

//
// insert new user vest info to db
//
func InsertUserVestInfo(info *types.AccountInfo) error {
	log := logs.GetLogger()
	if info == nil {
		log.Error("InsertUserVestInfo: fail to insert empty swap vest info")
		return errors.New("can't insert empty user vest info")
	}

	db,err := getServiceDB()
	if err != nil {
		log.Errorf("InsertUserVestInfo: fail to get db,the error is %v", err)
		return err
	}

	return db.Create(info).Error
}


//
// batch insert new user vest info to db
//
func BatchInsertUserVestInfo(list []*types.AccountInfo) error {
	logger := logs.GetLogger()
	length := len(list)
	if length < 1{
		log.Error("BatchInsertUserVestInfo: fail to insert empty swap vest info")
		return errors.New("can't insert empty user vest info list")
	}

	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("BatchInsertUserVestInfo: fail to get db,the error is %v", err)
		return err
	}
	sql := fmt.Sprintf("INSERT INTO %v (account_id, time, name, balance, vest, stake_vest_from_me) VALUES", config.GetExtraAccountInfoTableName())
	sign := 1
	for _,info := range list {
		if sign % batchInsertLimit == 0 {
			sql += fmt.Sprintf("('%s',%v,'%s',%v,%v,%v);", info.AccountId, info.Time, info.Name, info.Balance, info.Vest, info.StakeVestFromMe)
			_,err = db.DB().Exec(sql)
			if err != nil {
				logger.Errorf("BatchInsertUserVestInfo: fail to batch insert vote relations, the error is %v", err)
			}
			sign = 0
			sql = fmt.Sprintf("INSERT INTO %v (account_id, time, name, balance, vest, stake_vest_from_me) VALUES", config.GetExtraAccountInfoTableName())
		} else {
			sql += fmt.Sprintf("('%s',%v,'%s',%v,%v,%v),", info.AccountId, info.Time, info.Name, info.Balance, info.Vest, info.StakeVestFromMe)
		}
		sign++
	}
	if sign > 1 {
		sql = strings.TrimRight(sql, ",") + ";"
		_,err = db.DB().Exec(sql)
		if err != nil {
			logger.Errorf("BatchInsertUserVestInfo: fail to batch insert vote relations, the error is %v", err)
		}
	}
    return err
}

func CopyOldAccountInfoRecords(oldTable string, sTime int64, eTime int64) error {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("CopyOldAccountInfoRecords: fail to get db,the error is %v", err)
		return err
	}
	newTable := config.GetExtraAccountInfoTableName()
	fields := "account_id, time, name, balance, vest, stake_vest_from_me"
	s := fmt.Sprintf("INSERT INTO %v (%v) SELECT %v FROM %v WHERE time >= %v AND time <= %v", newTable, fields, fields, oldTable, sTime, eTime)
	_,err = db.DB().Exec(s)
	if err != nil {
		logger.Errorf("CopyOldAccountInfoRecords: fail to copy accounts info, the error is %v", err)
	}
	return err
}

func CopyOldVoteRelationRecords(oldTable string, sTime int64, eTime int64) error {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("CopyOldAccountInfoRecords: fail to get db,the error is %v", err)
		return err
	}
	newTable := config.GetExtraBpVoteRelationTableName()
	fields := "vote_id, voter, producer, time"
	s := fmt.Sprintf("INSERT INTO %v (%v) SELECT %v FROM %v WHERE time >= %v AND time <= %v", newTable, fields, fields, oldTable, sTime, eTime)
	_,err = db.DB().Exec(s)
	if err != nil {
		logger.Errorf("CopyOldAccountInfoRecords: fail to copy accounts info, the error is %v", err)
	}
	return err
}

//
// get bp vote relation from cos chain
//
func GetBpVoteRelation(t time.Time, officialBpList []string) ([]*types.BpVoteRelation, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetBpVoteRelation: fail to get observe node db, the error is %v", db)
		return nil, err
	}
	//filter := getDbFilterCondition(officialBpList, "producer")
	var (
		stateList []*plugins.ProducerVoteState
		rList []*types.BpVoteRelation

	)
	//err = db.Where(filter).Find(&stateList).Order("producer ASC").Error
	err = db.Find(&stateList).Order("producer ASC").Error
	if err != nil {
		logger.Errorf("GetBpVoteRelation: fail to get bp vote relation, the error is %v", err)
		return nil, err
	}
	for _,state := range stateList {
		relation := &types.BpVoteRelation{
			VoteId: utils.GenerateId(t, state.Producer, state.Voter), //timestamp + producer + voter for voteId
			Voter: state.Voter,
			Producer: state.Producer,
			Time: t.Unix(),
		}
		rList = append(rList, relation)
	}
	return rList, nil
}

//
// insert new bp vote relation to db
//
func InsertVoteRelation(relation *types.BpVoteRelation) error {
	logger := logs.GetLogger()
	if relation == nil {
		logger.Error("InsertVoteRelation: fail to insert empty vote relation")
		return errors.New("can't insert empty swap relation")
	}

	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("InsertVoteRelation: fail to get db,the error is %v", err)
		return err
	}
	return db.Create(relation).Error
}

//
// batch new bp vote relation to db
//
func BatchInsertVoteRelation(list []*types.BpVoteRelation) error {
	logger := logs.GetLogger()
	length := len(list)
	if length < 1 {
		log.Error("BatchInsertVoteRelation: fail to insert empty vote relation")
		return errors.New("can't insert empty swap relation")
	}

	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("BatchInsertVoteRelation: fail to get db,the error is %v", err)
		return err
	}
	sql := fmt.Sprintf("INSERT INTO %v (vote_id, voter, producer, time) VALUES", config.GetExtraBpVoteRelationTableName())
	sign := 1
	for _,relation := range list {
		if sign % batchInsertLimit == 0 {
			sql += fmt.Sprintf("('%s','%s','%s',%v);", relation.VoteId, relation.Voter, relation.Producer, relation.Time)
			_,err = db.DB().Exec(sql)
			if err != nil {
				logger.Errorf("BatchInsertVoteRelation: fail to batch insert vote relations, the error is %v", err)
			}
			sign = 0
			sql = fmt.Sprintf("INSERT INTO %v (vote_id, voter, producer, time) VALUES", config.GetExtraBpVoteRelationTableName())
		} else {
			sql += fmt.Sprintf("('%s','%s','%s',%v),", relation.VoteId, relation.Voter, relation.Producer, relation.Time)
		}
		sign++
	}
	if sign > 1 {
		sql = strings.TrimRight(sql, ",") + ";"
		_,err = db.DB().Exec(sql)
		if err != nil {
			logger.Errorf("BatchInsertVoteRelation: fail to batch insert vote relations, the error is %v", err)
		}
	}
	return err
}

//
// get bp vote record of voter from cos chain
//
func GetBpVoteRecords(t time.Time, officialBpList []string) ([]*types.BpVoteRecord, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetBpVoteRecords: fail to get observe node db, the error is %v", db)
		return nil, err
	}
	var (
		oriRecList []*plugins.ProducerVoteRecord
		recList []*types.BpVoteRecord
	)
	//filter := getDbFilterCondition(officialBpList, "producer")
	err = db.Find(&oriRecList).Order("producer ASC").Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetBpVoteRecords: fail to get bp vote records, the error is %v", err)
			return nil, err
		}
		return nil, nil
	}
	for _,rec := range oriRecList {
		record := &types.BpVoteRecord{
			VoteId: utils.GenerateId(t, rec.Voter, rec.Producer, strconv.FormatBool(rec.Cancel) + strconv.FormatUint(rec.BlockHeight, 10)),//timestamp + voter + producer + vote operation + block_height for voteId
			BlockHeight: rec.BlockHeight,
			BlockTime: rec.BlockTime,
			Voter: rec.Voter,
			Producer: rec.Producer,
			Cancel: rec.Cancel,
			Time: t.Unix(),
		}
		recList = append(recList, record)
	}
	return recList, nil
}

//
// insert bp vote record to db
//
func InsertBpVoteRecord(record *types.BpVoteRecord) error {
	log := logs.GetLogger()
	if record == nil {
		log.Error("InsertBpVoteRecord: fail to insert empty bp vote record")
		return errors.New("can't insert empty bp vote record")
	}

	db,err := getCosObserveNodeDb()
	if err != nil {
		log.Errorf("InsertBpVoteRecord: fail to get db,the error is %v", err)
		return err
	}
	return db.Create(record).Error
}

//
// Batch insert voter record
//
func BatchInsertVoteRecord(list []*types.BpVoteRecord) error {
	logger := logs.GetLogger()
	length := len(list)
	if length < 1{
		log.Error("BatchInsertVoteRecord: fail to insert empty bp vote record")
		return errors.New("can't insert empty bp vote record")
	}

	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("BatchInsertVoteRecord: fail to get db,the error is %v", err)
		return err
	}
	sql := "INSERT INTO bp_vote_records (vote_id, block_height, block_time, voter, producer, cancel, time) VALUES"
	for i,record := range list {
		if i + 1 == length {
			sql += fmt.Sprintf("('%s',%v,'%s','%s', '%s',%v, %v);", record.VoteId, record.BlockHeight, utils.ConvertTimeToStamp(record.BlockTime) , record.Voter, record.Producer, record.Cancel, record.Time)
		} else {
			sql += fmt.Sprintf("('%s',%v,'%s','%s', '%s',%v, %v),", record.VoteId, record.BlockHeight, utils.ConvertTimeToStamp(record.BlockTime), record.Voter, record.Producer, record.Cancel, record.Time)
		}
	}
	_,err = db.DB().Exec(sql)
	if err != nil {
		logger.Errorf("BactchInsertVoteRecord: fail to batch insert vote record, the error is %v", err)
	}
	return err
}

//
// Insert the distributed reward record into the database
//
func InsertRewardRecord(record *types.BpRewardRecord) error {
	log := logs.GetLogger()
	if record == nil {
		log.Error("InsertRewardRecord: fail to insert empty reward record")
		return errors.New("can't insert empty eward record")
	}

	db,err := getServiceDB()
	if err != nil {
		log.Errorf("InsertRewardRecord: fail to get db,the error is %v", err)
		return err
	}
	return db.Create(record).Error
}


func getDbFilterCondition(officialBpList []string, column string) string {
	filter := ""
	var filterList []string
	for _,name := range officialBpList {
		str := fmt.Sprintf("%v='%v'", column, name)
		filterList = append(filterList, str)
	}
	if len(filterList) > 0 {
		filter = strings.Join(filterList, " OR ")
	}
	return filter
}

//
// get all not success reward record
//
func GetAllNotSuccessRewardRecords() ([]*types.BpRewardRecord, error) {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("GetAllNotSuccessRewardRecords: fail to get db, the error is %v \n", err)
		return nil,err
	}
	var recs []*types.BpRewardRecord
	err = db.Find(&recs, "status = ? and transfer_hash != ?", types.ProcessingStatusDefault, "").Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Errorf("GetAllNotSuccessRewardRecords: fail to query fields, the error is %v", err)
			return nil, err
		}

	}
	return recs,err
}

//
// check transfer_to_vest trx is really success
//
func CheckTransferToVestTxIsSuccess(txHash string) (bool,error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("CheckTransferToVestTxIsSuccess: fail to get cos observe node db,the errors is %v", err)
		return false, err
	}
	var txInfo types.CosTrxInfo
	err = db.Table("trxinfo").Where("trx_id=?", txHash).Find(&txInfo).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false,err
	}

	var invoice types.CosTxInvoice
	err = json.Unmarshal([]byte(txInfo.Invoice), &invoice)
	if err != nil {
		logger.Errorf("Fail to unmarshal invoice of %v, the error is %v", txHash, err)
		return false, err
	} else {
		return invoice.Status == prototype.StatusSuccess, nil
	}

	return true,nil
}


//
// modify processed field
//
func MdRewardProcessStatus(id string, status int) error {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("MdRewardProcessStatus: fail to get db,the error is %v", err)
		return err
	}
	return db.Model(&types.BpRewardRecord{}).Where("id=?", id).Update("status", status).Error
}

//
// get voter's min vest on a week period
//
func GetVoterMinVestOfPeriod(usrName string, sTime int64, endTime int64) (uint64, error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetVoterMinVestOfPeriod: fail to get db,the error is %v", err)
		return 0, err
	}
	var rec types.AccountInfo
	err = db.Take(&rec).Where("vest = (SELECT min(vest) as min_vest FROM account_infos where name = ? AND time >= ? AND time <= ?)", usrName, sTime, endTime).Error
	if err != nil {
		logger.Errorf("GetVoterMinVestOfPeriod: fail to get min vest of %v on start:%v and end:%v", usrName, sTime, endTime)
		return 0,err
	}
	return rec.Vest, nil
}

func GetAllVoterOfBlkRange(sBlkNum uint64, eBlkNum uint64) ([]*plugins.ProducerVoteRecord, error) {
	logger := logs.GetLogger()
	nodeDb,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetAllVoterOfBlkRange: fail to get observe db,the error is %v", err)
		return nil, err
	}
	var curPeriodVoterList []*plugins.ProducerVoteRecord
	err = nodeDb.Select("DISTINCT(voter)").Where("block_height > ? AND block_height <= ?", sBlkNum, eBlkNum).Find(&curPeriodVoterList).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetAllRewardedVotersOfPeriodByBp: fail to get voters, the error is %v", err)
			return nil, err
		}
	}
	return curPeriodVoterList, nil
}

//
// get all voters which can get reward
//
func GetAllRewardedVotersOfPeriodByBp(bpName string, sTime int64, endTime int64, sBlkNum uint64, eBlkNum uint64, voterList []*plugins.ProducerVoteRecord) ([]*types.AccountInfo, error){
	logger := logs.GetLogger()
	db,err := getServiceDB()
	//db.LogMode(true)
	if err != nil {
		logger.Errorf("GetAllRewardedVotersOfPeriod: fail to get db,the error is %v", err)
		return nil, err
	}

	var (
		infoList []*types.AccountInfo
		finalList []*types.AccountInfo
	)
	voteRelationTable := config.GetExtraBpVoteRelationTableName()
	accountInfoTable := config.GetExtraAccountInfoTableName()
	//get all vote record from observe db
	joinSql := fmt.Sprintf("INNER JOIN (SELECT DISTINCT %v.voter FROM %v WHERE time > %v AND time <= %v AND %v.producer = '%v') as t1 ON %v.name = t1.voter", voteRelationTable, voteRelationTable, sTime, endTime, voteRelationTable, bpName, accountInfoTable)
	voterFilter := "''"
	listLen := len(voterList)
	if listLen > 0 {
		voterFilter = ""
		for idx,v := range voterList{
			voterFilter += fmt.Sprintf("'%v'",v.Voter)
			if idx + 1 < listLen {
				voterFilter += ","
			}
		}
		joinSql = fmt.Sprintf("INNER JOIN (SELECT DISTINCT %v.voter FROM %v WHERE time > %v AND time <= %v AND %v.producer = '%v' AND %v.voter NOT IN (%v)) as t1 ON %v.name = t1.voter", voteRelationTable, voteRelationTable, sTime, endTime, voteRelationTable, bpName, voteRelationTable, voterFilter, accountInfoTable)
	}
	groupSql := fmt.Sprintf("%v.name", accountInfoTable)
	selectSql := fmt.Sprintf("MIN(%v.vest) as vest,%v.name", accountInfoTable, accountInfoTable)
	whereSql := fmt.Sprintf("%v.time > %v AND %v.time <= %v", accountInfoTable, sTime, accountInfoTable,endTime)
	err = db.Table(accountInfoTable).Select(selectSql).Joins(joinSql).Where(whereSql).Group(groupSql).Order("name").Scan(&infoList).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetAllRewardedVotersOfPeriodByBp: fail to get voters, the error is %v", err)
			return nil, err
		}
		return nil, nil

	}
	//filter account which vest is less than utils.MinVoterDistributeVest
	for _,acct := range infoList {
		if acct.Vest >= utils.MinVoterDistributeVest {
			finalList = append(finalList, acct)
		}
	}

	return finalList, nil
}


func GetUserRewardHistory(acctName string, pageIndex int, pageSize int) ([]*types.BpRewardRecord, error, int) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetUserRewardHistory: fail to get db,the error is %v", err)
		return nil, errors.New("fail to get db service"), types.StatusGetDbError
	}
	var list []*types.BpRewardRecord
	err = db.Offset((pageIndex-1)*pageSize).Limit(pageSize).Where("voter = ?", acctName).Find(&list).Order("time").Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, errors.New("fail to query data"), types.StatusDbQueryError
		} else {
			return nil, errors.New("not found any record"), types.StatusNotFoundError
		}

	}
	return list, nil, types.StatusSuccess
}

//
// ger bp's reward history record of one period
//
func GetBpRewardHistoryByPeriod(period uint64, bpName string)  (*types.BpRewardRecord, error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetBpRewardHistoryByPeriod: fail to get db,the error is %v", err)
		return nil, errors.New("fail to get db service")
	}
	var rec types.BpRewardRecord
	err = db.Where("period = ? AND bp = ? AND reward_type = ?", period, bpName, types.RewardTypeToBp).First(&rec).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetBpRewardHistoryByPeriod: fail to get record history, the error is %v", err)
			return nil, errors.New("fail to get record")
		}
		return nil,nil
	}
	return &rec,nil
}

//
// get all bp's reward history of one period
//
func GetAllBpRewardHistoryByPeriod(period uint64) ([]*types.BpRewardRecord, error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetAllBpRewardHistoryByPeriod: fail to get db,the error is %v", err)
		return nil, errors.New("fail to get db service")
	}
	var list []*types.BpRewardRecord
	err = db.Where("period = ? AND reward_type = ?", period, types.RewardTypeToBp).Order("annualized_rate desc").Find(&list).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetAllBpRewardHistoryByPeriod: fail to get record history, the error is %v", err)
			return nil, errors.New("fail to get record")
		}
		return nil,nil
	}
	return list,nil
}

func GetAllBpRewardHistoryByPeriodRange(start uint64, end uint64) ([]*types.BpRewardRecord, error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetAllBpRewardHistoryByPeriodRange: fail to get db,the error is %v", err)
		return nil, errors.New("fail to get db service")
	}
	var list []*types.BpRewardRecord
	err = db.Where("reward_type = ? AND period >= ? AND period <= ?", types.RewardTypeToBp, start, end).Order("period asc,annualized_rate desc").Find(&list).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetAllBpRewardHistoryByPeriodRange: fail to get record history of period range(start:%v,end:%v), the error is %v", err, start, end)
			return nil, errors.New("fail to get record")
		}
		return nil, nil
	}
	return list,nil
}

//
// get the lib number
//
func GetLib() (uint64, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetLib: fail to get cos observe node db,the errors is %v", err)
		return 0, err
	}
	var (
		libInfo types.LibInfo
	)

	err = db.Table("libinfo").Find(&libInfo).Error
	if err != nil {
		logger.Errorf("GetLib: fail to get lib, the error is %v", err)
		return 0,err
	}
	return libInfo.Lib,nil
}

//func CreateBlockLog(log *iservices.BlockLogRecord) error {
//	db,err := getCosObserveNodeDb()
//	if err != nil {
//		fmt.Printf("CreateBlockLog: fail to get cos observe db, the error is %v \n", err)
//		return  err
//	}
//	tName := log.TableName()
//	if !db.HasTable(tName) {
//		err = db.CreateTable(log).Error
//		if err != nil {
//			return  err
//		}
//	}
//	//return db.FirstOrCreate(log).Error
//	return nil
//}

//
// get block log by block number
//
func GetBlockLogByNum(blkNum uint64) (*iservices.BlockLogRecord, error) {
	logger := logs.GetLogger()
	db,err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetBlockInfoByNum: fail to get cos observe node db,the errors is %v", err)
		return nil, err
	}
	var rec iservices.BlockLogRecord
	tabName := iservices.BlockLogTableNameForBlockHeight(blkNum)
	err = db.Table(tabName).Where("block_height = ? AND final = ?", blkNum, 1).Find(&rec).Error
	if err != nil {
		logger.Errorf("GetBlockInfoByNum: fail to block record of block:%v, the error is %v", blkNum, err)
		return nil,err
	}
	return &rec,nil
}

//
// calculate total generated block number of bp during a period
//
func CalcBpTotalBlockNumOfRange(bp string, start uint64, end uint64) (uint64, error) {
	logger := logs.GetLogger()
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetBlockLogById: fail to get cos observe db, the error is %v \n", err)
		return 0,err
	}
	sTabName := iservices.BlockLogTableNameForBlockHeight(start)
	eTabName := iservices.BlockLogTableNameForBlockHeight(end)
	var (
		total uint64
	)

	err = db.Table(sTabName).Where("block_producer = ? AND block_height > ? AND block_height <= ? AND final = ?", bp, start, end, 1).Count(&total).Error
    if err != nil {
    	return 0, err
	}
	if sTabName != eTabName {
		//all record not in one  db table, calculate total number in another table
		var left uint64
		err = db.Table(eTabName).Where("block_producer = ? AND block_height > ? AND block_height <= ? AND final=?", bp, start, end, 1).Count(&left).Error
		if err != nil {
			return 0,err
		}
		total += left
	}
	return total,nil
}

//
// get latest reward period in db
//
func GetLatestDistributedPeriod(isMax bool) (uint64, error) {
	logger := logs.GetLogger()
	db, err := getServiceDB()
	if err != nil {
		logger.Errorf("GetLatestDistributedPeriod: fail to get cos observe db, the error is %v ", err)
		return 0,err
	}
    //var rewardRec types.BpRewardRecord
	var (
		period uint64
		total int
	)

	err = db.Model(types.BpRewardRecord{}).Select("count(*)").Row().Scan(&total)
	if err != nil {
		logger.Errorf("GetLatestDistributedPeriod: fail to check table is empty", err)
		return 0, err
	} else {
		if total == 0 {
			//table is empty
			return 0, nil
		}
	}
	filter := "max(period)"
	if !isMax {
		filter = "min(period)"
	}
	err = db.Model(types.BpRewardRecord{}).Select(filter).Row().Scan(&period)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Errorf("GetLatestDistributedPeriod: fail to get latest distributed period, the error is %v", err)
			return 0,err
		} else {
			logger.Error("GetLatestDistributedPeriod: fail to find latest distributed period")
			return 0, nil
		}
	}
	return period,nil
}

//
// get total number of blocks generated by every bp of a distribute period
//
func CalcBpGeneratedBlocksOnOnePeriod(start uint64, end uint64) ([]*types.BpBlockStatistics, error) {
	logger := logs.GetLogger()
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("CalcBpGeneratedBlocksOnOnePeriod: fail to get cos observe db, the error is %v \n", err)
		return nil,err
	}
	//db.LogMode(true)
	sTabName := iservices.BlockLogTableNameForBlockHeight(start)
	eTabName := iservices.BlockLogTableNameForBlockHeight(end)
	sIdx := "idx_" + sTabName + "_block_height"
	var list []*types.BpBlockStatistics
	sql := fmt.Sprintf("select count(*) as total_count, block_producer from %v FORCE INDEX(%v) where block_height > %v and block_height <= %v and final = %v GROUP BY block_producer ORDER BY total_count", sTabName, sIdx, start, end, 1)
	if sTabName != eTabName {
		// need select union two table
		eIdx := "idx_" + eTabName + "_block_height"
		sql = fmt.Sprintf("SELECT COUNT(*) as total_count , block_producer from (select block_producer from %v FORCE INDEX(%v) where block_height > %v and block_height <= %v and final = %v union all (select block_producer from %v FORCE INDEX(%v) where block_height > %v and block_height <= %v and final = %v)) as t GROUP BY block_producer ORDER BY total_count", sTabName, sIdx, start, end, 1, eTabName, eIdx, start, end,1)
	}
	err = db.Raw(sql).Scan(&list).Error
	if err != nil {
		logger.Errorf("CalcBpGeneratedBlocksOnOnePeriod: fail to get bp's block statistics info, the error is %v", err)
		return nil, err
	}
	return list,nil
}

//
// get all bp from chain 
//
func GetAllBpFromChain() ([]*plugins.ProducerVoteState,error) {
	logger := logs.GetLogger()
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetAllBpFromChain: fail to get cos observe db, the error is %v \n", err)
		return nil,err
	}
	var list []*plugins.ProducerVoteState
	err = db.Model(plugins.ProducerVoteState{}).Select("DISTINCT producer").Scan(&list).Error
	if err != nil {
		logger.Errorf("GetAllBpFromChain: fail to get all bp, the error is %v", err)
		return nil, err
	}
	return list, nil
}

//
// calculate total voters number on cos chain
//
func CalcTotalVotersNumber() (uint64, error) {
	logger := logs.GetLogger()
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("CalcTotalVotersNumber: fail to get cos observe db, the error is %v", err)
		return 0,err
	}
	var total uint64
	err = db.Model(&plugins.ProducerVoteState{}).Count(&total).Error
	if err != nil {
		logger.Errorf("CalcTotalVotersNumber: fail to calculate total voters count, the error is %v", err)
		return 0,err
	}
	return total,nil
}

//
// calculate total block producer on cos chain
//
func CalcTotalBpNumber() (uint64, error) {
	logger := logs.GetLogger()
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("CalcTotalBpNumber: fail to get cos observe db, the error is %v", err)
		return 0,err
	}
	var total uint64
	err = db.Model(&plugins.ProducerVoteState{}).Select("COUNT(DISTINCT(producer))").Row().Scan(&total)
	if err != nil {
		logger.Errorf("CalcTotalBpNumber: fail to calculate total voters count, the error is %v", err)
		return 0,err
	}
	return total,nil
}

//
// get max RIO
//
func GetMaxROIOfBpReward() (float64,error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetMaxROIOfBpReward: fail to get db,the error is %v", err)
		return 0,err
	}
	var (
		maxROI float64
		total int
	)
	err = db.Model(types.BpRewardRecord{}).Select("count(*)").Row().Scan(&total)
	if err != nil {
		logger.Errorf("GetLatestDistributedPeriod: fail to check table is empty", err)
		return 0, err
	} else {
		if total == 0 {
			//table is empty
			return 0, nil
		}
	}
	err = db.Model(types.BpRewardRecord{}).Select("MAX(annualized_rate)").Where("reward_type = ?", types.RewardTypeToVoter).Row().Scan(&maxROI)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Errorf("GetMaxROIOfBpReward: fail to get max ROI of bp reward, the error is %v", err)
			return 0,err
		}
		return 0,nil
	}
	return maxROI,nil
}

func GetAllRewardRecordsOfOnePeriod(period uint64) ([]*types.BpRewardRecord, error) {
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("GetAllRewardRecordsOfOnePeriod: fail to get db,the error is %v", err)
		return nil,err
	}
	var list []*types.BpRewardRecord
	err = db.Where("period=?", period).Find(&list).Error
	if err != nil {
		logger.Errorf("GetAllRewardRecordsOfOnePeriod: fail to get all reward record of period:%v,the error is %v", period, err)
		return nil,err
	}
	return list, nil
}

func MdRewardRecord(rec *types.BpRewardRecord) error {
	if rec == nil {
		return errors.New("can't update empty reward record")
	}
	logger := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		logger.Errorf("MdRewardRecord: fail to get db,the error is %v", err)
		return err
	}
	return db.Save(rec).Error
}

// Get official bp's gift ticket reward on block range
func GetGiftRewardOfOfficialBpOnRange(sBlkNum,eBlkNum uint64) ([]*types.GiftTicketRewardInfo, error) {
	logger := logs.GetLogger()
	prefix := config.GetGiftRewardBpNamePrefix()
	receiveAcct := config.GetTicketRewardReceiveAccount()
	if len(prefix) < 1 {
		logger.Errorf("GetGiftRewardOfOfficialBpOnRange: bp name prefix is empty,block range is(start:%v,end:%v)", sBlkNum, eBlkNum)
		return nil, errors.New("can't get gift reward with empty bp name prefix")
	}
	if len(receiveAcct) < 1 {
		logger.Errorf("GetGiftRewardOfOfficialBpOnRange: ticket reward deposit account is empty,block range is(start:%v,end:%v)", sBlkNum, eBlkNum)
		return nil, errors.New("can't get gift reward with empty ticket reward deposit account")
	}
	db, err := getCosObserveNodeDb()
	if err != nil {
		logger.Errorf("GetUserVestInfo: fail to get observe node db, the error is %v", db)
		return nil, err
	}
	var rewardList []*types.GiftTicketRewardInfo
	queryPrefix := prefix + "[1-9][0-9]*-[1-9][0-9]*$"

	err = db.Model(plugins.TransferRecord{}).Select("SUM(amount) as total_amount,SUBSTRING_INDEX(memo,'-',1) as bp").Where("block_height > ? AND block_height <= ? AND `to` = ? AND memo REGEXP ?", sBlkNum, eBlkNum, receiveAcct, queryPrefix).Group("bp").Scan(&rewardList).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return rewardList, nil
}

func GetAllValidVotersOfBp(bp string, sBlkNum uint64, eBlkNum uint64, sBlkTime int64, eBlkTime int64) ([]*types.AccountInfo, []*plugins.ProducerVoteRecord,error) {
	//1.get all voter in current block range
	curPeriodVoters,err := GetAllVoterOfBlkRange(sBlkNum, eBlkNum)
	if err != nil {
		fmt.Printf("GetAllValidVotersOfBp: fail to get voter of current period")
		return nil, nil,err
	}
	svDb,err := getServiceDB()
	if err != nil {
		fmt.Printf("GetAllValidVotersOfBp:fail to get service bp, the error is %v \n", err)
		return nil,nil, err
	}

	//2. get bp's all voter with min vest
	var (
		allVoters []*types.AccountInfo
		finalList []*types.AccountInfo
	)
    svDb.LogMode(true)
	sql := fmt.Sprintf("SELECT MIN(vest) as vest,name FROM %v WHERE `name` in (SELECT DISTINCT `name`  FROM %v WHERE `name` in (SELECT DISTINCT voter FROM %v WHERE producer = \"%v\" AND time > %v AND time <= %v)) AND time > %v AND time <= %v GROUP BY `name` ORDER BY `name`", config.GetExtraAccountInfoTableName(), config.GetExtraAccountInfoTableName(), config.GetExtraBpVoteRelationTableName(),bp, sBlkTime, eBlkTime, sBlkTime, eBlkTime)
	err = svDb.Raw(sql).Scan(&allVoters).Error
	if err != nil {
		fmt.Printf("GetAllValidVotersOfBp: fail to get all voters, the error is %v \n", err)
	}
	for _,origin := range allVoters {
		isCurPeriodVoter := false
		//filter all voter which has vote record in this period
		for _,curPeriodVoter := range curPeriodVoters {
			if origin.Name == curPeriodVoter.Voter {
				isCurPeriodVoter = true
				break
			}
		}
		if !isCurPeriodVoter && origin.Vest >= 10000000 {
			finalList = append(finalList, origin)
		}
	}
	return finalList, curPeriodVoters, nil

}

func CheckOnePeriodIsDistribute(period uint64) (bool,error) {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("CheckOnePeriodIsDistribute: fail to get db,the error is %v", err)
		return false,err
	}
	var cnt int
	err = db.Model(types.BpRewardRecord{}).Select("count(*)").Where("period = ?", period).Row().Scan(&cnt)
	if err != nil && err != gorm.ErrRecordNotFound{
		return false,err
	}
	isDistribute := false
	if cnt > 0 {
		isDistribute = true
	}
	return isDistribute,nil
}

func CheckOnePeriodBpOrVoterIsDistribute(isToBp bool,period uint64) (bool,error)  {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("CheckOnePeriodBpOrVoterIsDistribute: fail to get db,the error is %v", err)
		return false,err
	}
	var cnt int
	filter := fmt.Sprintf("period = %v AND reward_type = %v", period, types.RewardTypeToVoter)
	if isToBp {
		filter = fmt.Sprintf("period = %v AND reward_type = %v", period, types.RewardTypeToBp)
	}

	err = db.Model(types.BpRewardRecord{}).Select("count(*)").Where(filter).Row().Scan(&cnt)
	if err != nil && err != gorm.ErrRecordNotFound {
		return false,err
	}
	isDistribute := false
	if cnt > 0 {
		isDistribute = true
	}
	return isDistribute,nil
}

func GetRewardRecordById(id string) (*types.BpRewardRecord,error) {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("GetRewardRecordById: fail to get db,the error is %v", err)
		return nil,err
	}
	var rec types.BpRewardRecord
	err = db.Model(types.BpRewardRecord{}).Where("id = ?", id).Find(&rec).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Errorf("GetRewardRecordById: fail to get db,the error is %v", err)
			return nil, err
		}
	}
	return &rec,nil
}

func GetVoterStatisticsDataOfPeriod(period uint64) (*types.HistoricalVotingData, error) {
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("GetVoterStatisticsDataOfPeriod: fail to get db,the error is %v", err)
		return nil,err
	}
	var (
		voterCnt int
		maxROI = "0"
	)
	//get total voter count
	err = db.Model(types.BpRewardRecord{}).Select("COUNT(voter)").
		Where("period = ? AND reward_type = ?", period, types.RewardTypeToVoter).Row().Scan(&voterCnt)
	if err != nil {
		log.Errorf("GetVoterStatisticsDataOfPeriod: fail to get total voter of period %v,the error is %v", period, err)
		return nil, err
	}
	//get max ROI
	bpFilter := "''"
	listLen := len(config.OfficialBpList)
	filterSql := fmt.Sprintf("period = %v AND reward_type = %v", period, types.RewardTypeToBp)
	if listLen > 0 {
		bpFilter = ""
		for idx,v := range config.OfficialBpList{
			bpFilter += fmt.Sprintf("'%v'",v)
			if idx + 1 < listLen {
				bpFilter += ","
			}
		}
		filterSql = fmt.Sprintf("period = %v AND reward_type = %v AND bp in (%v)", period, types.RewardTypeToBp, bpFilter)

	}
    err =  db.Model(types.BpRewardRecord{}).Select("MAX(annualized_rate)").Where(filterSql).Row().Scan(&maxROI)
    if err != nil {
		log.Errorf("GetVoterStatisticsDataOfPeriod: fail to get max ROI of period %v,the error is %v", period, err)
		return nil, err
	}
    data := &types.HistoricalVotingData{
    	VotersNumber: strconv.Itoa(voterCnt),
    	MaxROI: maxROI,
	}
    return data,nil
}

func GetAccountInfoRecordsByRangeTime(tName string, sTime int64, eTime int64) ([]*types.AccountInfo, error) {
	if !utils.CheckIsNotEmptyStr(tName) {
		return nil, errors.New("table name is empty")
	}
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("GetAccountInfoRecordsByRangeTime: fail to get db,the error is %v", err)
		return nil,err
	}
	var infoList []*types.AccountInfo
	sql := fmt.Sprintf("time >= %v", sTime)
	if eTime >= sTime {
		sql += fmt.Sprintf(" AND time <= %v", eTime)
	}
	rows,err := db.Table(tName).Select("*").Where(sql).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var info types.AccountInfo
		err = db.ScanRows(rows, &info)
		if err != nil {
			return nil, err
		} else {
			infoList = append(infoList, &info)
		}
	}
	return infoList, nil
}

func GetVoteRelationsByRangeTime(tName string, sTime int64, eTime int64) ([]*types.BpVoteRelation, error) {
	if !utils.CheckIsNotEmptyStr(tName) {
		return nil, errors.New("table name is empty")
	}
	log := logs.GetLogger()
	db,err := getServiceDB()
	if err != nil {
		log.Errorf("GetVoteRelationsByRangeTime: fail to get db,the error is %v", err)
		return nil,err
	}
	var relationList []*types.BpVoteRelation
	sql := fmt.Sprintf("time >= %v", sTime)
	if eTime >= sTime {
		sql += fmt.Sprintf(" AND time <= %v", eTime)
	}
	rows,err := db.Table(tName).Select("*").Where(sql).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var info types.BpVoteRelation
		err = db.ScanRows(rows, &info)
		if err != nil {
			return nil, err
		} else {
			relationList = append(relationList, &info)
		}
	}
	return relationList, nil
}
