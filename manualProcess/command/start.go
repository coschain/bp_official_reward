package command

import (
	"bp_official_reward/config"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"encoding/json"
	"fmt"
	"github.com/coschain/cobra"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"io/ioutil"
	"os"
)


type failRec struct {
	RecId  string   `json:"recId"`
	TransferTx string `json:"transferTx"`
	Status   int   `json:"status"`
}

var StartCmd = func() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "start",
		Short:     "start failed cos bp reward distribution process service",
		Long:      "start failed cos bp reward distribution process service,if has arg 'env',will use it for service env",
		ValidArgs: []string{"env"},
		Run:       startService,
	}
	cmd.Flags().StringVarP(&svEnv, "env", "e", "pro", "service env (default is pro)")

	return cmd
}

func startService(cmd *cobra.Command, args []string)  {
	err := config.SetConfigEnv(svEnv)
	if err != nil {
		fmt.Printf("StartService:fail to set env \n")
		os.Exit(1)
	}

	//load config json file
	err = config.LoadRewardConfig("../../bp_reward.json")
	if err != nil {
		fmt.Println("StartService:fail to load config file ")
		os.Exit(1)
	}

	//load fail swap records file
	recJson,err := ioutil.ReadFile(config.GetFailDistributeFilePath())
	if err != nil {
		fmt.Printf("fail to load failed reward records, the error is %v \n", err)
		os.Exit(1)
	} else {
		var recs []failRec
		err := json.Unmarshal(recJson, &recs)
		if err != nil {
			fmt.Printf("fail to Unmarshal records, the error is %v \n", err)
			os.Exit(1)
			return
		} else {
			fmt.Printf("records number is %v \n", len(recs))
			dbCfg, err := config.GetRewardDbConfig()
			if err != nil {
				fmt.Printf("fail to get reward db config")
				os.Exit(1)
				return
			}
			//open db
			source := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DbName)
			db, err := gorm.Open(dbCfg.Driver, source)
			if err != nil {
				fmt.Printf("fail to open db, the error is %v \n", err)
				os.Exit(1)
				return
			}
			defer db.Close()
			for _, rec := range recs {
				if utils.CheckIsNotEmptyStr(rec.RecId) {
					if rec.Status == types.ProcessingStatusFail || rec.Status == types.ProcessingStatusNotNeedTransfer {
						var err error
						var rewardRec types.BpRewardRecord
						err = db.Take(&rewardRec, "id=?", rec.RecId).Error
						if err != nil {
							fmt.Printf("fail to get record,the error is %v \n", err)
							continue
						}
						if utils.CheckIsNotEmptyStr(rec.TransferTx) {
							err = db.Model(&types.BpRewardRecord{}).Where("id=?", rec.RecId).Updates(map[string]interface{}{"status": rec.Status, "transfer_tx_hash": rec.TransferTx}).Error
						} else {
							err = db.Model(&types.BpRewardRecord{}).Where("id=?", rec.RecId).Update("status", rec.Status).Error
						}
						if err != nil {
							fmt.Printf("fail to modify rec %v, the error is %v \n", rec, err)
						} else {
							fmt.Printf("succcess modify rec %v \n", rec)
						}
					}
				} else {
					fmt.Printf("can't modify invalid rec %v \n", rec)
				}
			}
		}
	}

	os.Exit(0)
}
