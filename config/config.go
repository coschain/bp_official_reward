package config

import (
	"encoding/json"
	"errors"
	"fmt"
	cosUtil "github.com/coschain/contentos-go/common"
	"github.com/coschain/contentos-go/prototype"
	"io/ioutil"
	"strconv"
	"sync"
)

const (
	EnvDev  = "dev"
	EnvPro  = "pro"
	EnvTest = "test"
)


type BlockLogDbInfo struct {
	BlockLogDbDriver   string `json:"blockLogDbDriver"`
	BlockLogDbUser     string `json:"blockLogDbUser"`
	BlockLogDbPassword string `json:"blockLogPassword"`
	BlockLogDbName     string `json:"blockLogDbName"`
	BlockLogDbHost     string `json:"blockLogDbHost"`
	BlockLogDbPort     string `json:"blockLogDbPort"`
}

type ExtraDistributeBpInfo struct {
	BpName string `json:"bpName"`
	DistributeStartNum uint64 `json:"distributeStartNum"`
	SnapShotStartNum   uint64 `json:"snapShotStartNum"`
}

type EnvConfig struct {
	CosSenderAcct   string    `json:"cosSenderAcct"`
	CosSenderPrivKey string   `json:"cosSenderPrivKey"`
	HttpPort      string      `json:"httpPort"`
	DbDriver      string      `json:"DbDriver"`
	DbUser        string      `json:"dbUser"`
	DbPassword    string      `json:"dbPassword"`
	DbName        string      `json:"dbName"`
	DbHost        string      `json:"dbHost"`
	DbPort        string      `json:"dbPort"`
	NodeAddrList  []string    `json:"nodeAddrList"`
	LogPath         string    `json:"logPath"`
	BlockLogDbList  []BlockLogDbInfo `json:"blockLogDbList"`
	FailDistributeFilePath string `json:"failDistributeFilePath"`
	ReprocessFilePath string `json:"reprocessFilePath"`
	RewardBpList  []string    `json:"rewardBpList"`
	SnapshotInterval string `json:"snapshotInterval"`
	DistributeInterval string `json:"distributeInterval"`
	DistributeTimeInterval string `json:"distributeTimeInterval"`
	ServiceStarPeriodBlockNum string `json:"serviceStarPeriodBlockNum"`
	CacheTimeInterval  string `json:"cacheTimeInterval"`
	TicketBpMemoPrefix string `json:"ticketBpMemoPrefix"`
	TicketReceiveAccount string `json:"ticketReceiveAccount"`
	ExtraAccountsInfoTableName string `json:"extraAccountsInfoTableName"`
	ExtraVoteRelationTableName string `json:"extraVoteRelationTableName"`
	ExtraRewardBpList []*ExtraDistributeBpInfo `json:"extraRewardBpList"`
}

type RewardConfig struct {
	Dev    EnvConfig     `json:"dev"`
	Pro    EnvConfig     `json:"pro"`
	Test   EnvConfig     `json:"test"`
}

type DbConfig struct {
	Driver string
	User   string
	Password string
	Host     string
	Port     string
	DbName   string
}


var (
	rewardConfig *EnvConfig
	configOnce sync.Once
	env = EnvDev // default env is dev
	httpPort = "8000" //default http port of web server
	ServiceStarPeriodBlockNum uint64
	DistributeTimeInterval int64
	DistributeInterval uint64
	SnapshotTimeInterval int64
	CacheTimeInterval int64
	OfficialBpList []string
)


// read config json file
func LoadRewardConfig(path string) error {
	if rewardConfig != nil {
		return nil
	}
	var config RewardConfig
	cfgJson,err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("LoadRewardConfig:fail to read json file, the error is %v", err)
		return err
	} else {
		if err := json.Unmarshal(cfgJson, &config); err != nil {
			fmt.Printf("LoadRewardConfig: fail to  Unmarshal json, the error is %v \n", err)
		} else {
			if IsDevEnv() {
				rewardConfig = &config.Dev
			} else if IsTestEnv() {
				rewardConfig = &config.Test
			} else if IsProEnv(){
				rewardConfig = &config.Pro
			} else {
				return errors.New("fail to get reward config of unKnown env")
			}

			SnapshotTimeInterval,err = strconv.ParseInt(rewardConfig.SnapshotInterval, 10, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("fail to convert SnapshotInterval:%v to int64, the error is %v", rewardConfig.SnapshotInterval, err))
			}
			DistributeTimeInterval,err = strconv.ParseInt(rewardConfig.DistributeTimeInterval , 10, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("fail to convert DistributeTimeInterval:%v to int64, the error is %v", rewardConfig.DistributeTimeInterval, err))
			}
			DistributeInterval,err = strconv.ParseUint(rewardConfig.DistributeInterval, 10, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("fail to convert DistributeInterval:%v to int64, the error is %v", rewardConfig.DistributeInterval, err))
			}

			ServiceStarPeriodBlockNum,err = strconv.ParseUint(rewardConfig.ServiceStarPeriodBlockNum, 10, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("fail to convert ServiceStarPeriodBlockNum:%v to int64, the error is %v", rewardConfig.ServiceStarPeriodBlockNum, err))
			}
			CacheTimeInterval,err = strconv.ParseInt(rewardConfig.CacheTimeInterval, 10, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("fail to convert CacheTimeInterval:%v to int64, the error is %v", rewardConfig.ServiceStarPeriodBlockNum, err))
			}
			OfficialBpList = rewardConfig.RewardBpList

			if len(rewardConfig.TicketReceiveAccount) < 1 || !CheckCosAccountIsValid(rewardConfig.TicketReceiveAccount){
				return errors.New("ticket receive account is invalid")
			}

			if len(rewardConfig.TicketBpMemoPrefix) < 1 {
				return errors.New("ticket gift transfer memo prefix is invalid")
			}
		}
	}
	return nil
}

func SetConfigEnv(ev string) error{
	if ev != EnvPro && ev != EnvDev && ev != EnvTest {
		return errors.New(fmt.Sprintf("Fail to set unknown environment %v", ev))
	}
	configOnce.Do(func() {
		env = ev
	})
	return nil
}

func IsDevEnv() bool {
	return env == EnvDev
}

func IsTestEnv() bool {
	return env == EnvTest
}

func IsProEnv() bool {
	return env == EnvPro
}

func GetHttpPort() string {
	return rewardConfig.HttpPort
}

// get log output path 
func GetLogOutputPath() string {
	if rewardConfig != nil {
		return rewardConfig.LogPath
	}
	return ""
}

func GetFailDistributeFilePath() string {
	if rewardConfig != nil {
		return rewardConfig.FailDistributeFilePath
	}
	return ""
}

func GetReprocessFilePath() string {
	if rewardConfig != nil {
		return rewardConfig.ReprocessFilePath
	}
	return ""
}

// get cos observe node database config list
func GetCosObserveNodeDbConfigList() ([]*DbConfig, error) {
	var list []*DbConfig
	if rewardConfig != nil {
		for _,cf := range rewardConfig.BlockLogDbList {
			info := &DbConfig{}
			info.Driver = cf.BlockLogDbDriver
			info.User = cf.BlockLogDbUser
			info.Password = cf.BlockLogDbPassword
			info.Port= cf.BlockLogDbPort
			info.Host = cf.BlockLogDbHost
			info.DbName = cf.BlockLogDbName
			list = append(list, info)
		}

	} else {
		return nil, errors.New("can't get observe db config from empty reward config")
	}
	return list,nil
}

// get local reward service db config
func GetRewardDbConfig() (*DbConfig,error) {
	var config *DbConfig
	if rewardConfig != nil {
		config = &DbConfig{}
		config.Driver = rewardConfig.DbDriver
		config.User = rewardConfig.DbUser
		config.Password = rewardConfig.DbPassword
		config.Port= rewardConfig.DbPort
		config.Host = rewardConfig.DbHost
		config.DbName = rewardConfig.DbName
	} else {
		return nil, errors.New("can't get service db config from empty reward config")
	}
	return config, nil
}

// get cos chain all rpc address
func GetCosRpcAddrList() ([]string,error) {
	if rewardConfig == nil {
		return nil, errors.New("can't get rpc address list from empty config")
	}
	return rewardConfig.NodeAddrList,nil
}

func GetRpcTimeout() int  {
	return 3
}

// get cos chain id
func GetCosChainId() uint32  {
	cId := "main"
	if IsDevEnv() {
		cId = "dev"
	} else if IsTestEnv() {
		cId = "test"
	}
	return cosUtil.GetChainIdByName(cId)
}

// get address and private key of our cos transfer account
func GetMainNetCosSenderInfo() (addr string, privKey string) {
	return rewardConfig.CosSenderAcct,rewardConfig.CosSenderPrivKey
}

// Get prefix of transfer memo in a ticket reward transfer
func GetGiftRewardBpNamePrefix() string {
	return rewardConfig.TicketBpMemoPrefix
}

func GetTicketRewardReceiveAccount() string {
	return rewardConfig.TicketReceiveAccount
}

func CheckCosAccountIsValid(addr string) bool {
	acct := prototype.AccountName{Value: addr}
	if err := acct.Validate(); err != nil {
		return false
	}
	return true
}

func GetExtraAccountInfoTableName() string {
	tName := rewardConfig.ExtraAccountsInfoTableName
	if tName != "" && len(tName) > 0 {
		return tName
	}
	return  "account_infos"
}

func GetExtraBpVoteRelationTableName() string {
	tName := rewardConfig.ExtraVoteRelationTableName
	if tName != "" && len(tName) > 0 {
		return rewardConfig.ExtraVoteRelationTableName
	}
	return "bp_vote_relations"
}

func GetExtraRewardBpInfoList() []*ExtraDistributeBpInfo {
	if rewardConfig != nil {
		return rewardConfig.ExtraRewardBpList
	}
	return nil
}

func CheckExtraBpCanDistribute(bpName string, curBlkNum uint64) bool {
	if len(bpName) > 0 {
		bpList := GetExtraRewardBpInfoList()
		if bpList != nil && len(bpList) > 0 {
			for _,info := range bpList{
				if info.BpName == bpName && curBlkNum >= info.DistributeStartNum {
					return true
				}
			}
		}
	}
	return false
}

func GetAllExtraRewardBpNameList() []string {
	bpList := GetExtraRewardBpInfoList()
	var nameList []string
	if bpList != nil && len(bpList) > 0 {
		for _,info := range bpList{
			if len(info.BpName) > 0 {
				nameList = append(nameList, info.BpName)
			}
		}
	}
	return nameList
}

func GetAllCanDistributeExtraRewardBpNameList(curBlkNum uint64) []string {
	var nameList []string
	bpList := GetExtraRewardBpInfoList()
	if bpList != nil && len(bpList) > 0 {
		for _,info := range bpList{
			if curBlkNum >= info.DistributeStartNum && len(info.BpName) > 0 {
				nameList = append(nameList, info.BpName)
			}
		}
	}
	return nameList
}