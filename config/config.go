package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	cosUtil "github.com/coschain/contentos-go/common"
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
)


// read config json file
func LoadRewardConfig() error {
	if rewardConfig != nil {
		return nil
	}
	var config RewardConfig
	cfgJson,err := ioutil.ReadFile("../../../bp_reward.json")
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
