package command

import (
	"bp_official_reward/config"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"encoding/json"
	"fmt"
	"github.com/coschain/cobra"
	"github.com/jinzhu/gorm"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"os"
)


type failRec struct {
	RecId  string   `json:"recId"`
	TransferTx string `json:"transferTx"`
	Status   int   `json:"status"`
}

var svEnv string

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
	var rewardConfig config.RewardConfig
	cfgJson,err := ioutil.ReadFile("../../bp_reward.json")
	if err != nil {
		fmt.Printf("LoadSwapConfig:fail to read json file, the error is %v \n", err)
	} else {
		if err := json.Unmarshal(cfgJson, &rewardConfig); err != nil {
			fmt.Printf("LoadSwapConfig: fail to  Unmarshal json, the error is %v", err)
			os.Exit(1)
		} else {
			env := rewardConfig.Dev
			if svEnv == config.EnvTest {
				env = rewardConfig.Test
			} else if svEnv == config.EnvPro {
				env = rewardConfig.Pro
			}
			//load fail swap records file
			recJson,err := ioutil.ReadFile(env.FailDistributeFilePath)
			if err != nil {
				fmt.Printf("fail to load failed swap records, the error is %v \n", err)
			} else {
				var recs []failRec
				err := json.Unmarshal(recJson, &recs)
				if err != nil {
					fmt.Printf("fail to Unmarshal records, the error is %v \n", err)
					os.Exit(1)
					return
				} else {
					fmt.Printf("records number is %v \n", len(recs))
					//open db
					source := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", env.DbUser, env.DbPassword, env.DbHost, env.DbPort, env.DbName)
					db,err := gorm.Open(env.DbDriver, source)
					if err != nil {
						fmt.Printf("fail to open db, the error is %v \n",err)
						os.Exit(1)
						return
					}
					defer db.Close()
					for _,rec := range recs {
						if  utils.CheckIsNotEmptyStr(rec.RecId) {
							if rec.Status == types.ProcessingStatusFail || rec.Status == types.ProcessingStatusNotNeedTransfer {
								var err error
								var rewardRec types.BpRewardRecord
								err = db.Take(&rewardRec, "id=?", rec.RecId).Error
								if err != nil {
									fmt.Printf("fail to get record,the error is %v \n", err)
									continue
								}
								if utils.CheckIsNotEmptyStr(rec.TransferTx) {
									err = db.Model(&types.BpRewardRecord{}).Where("id=?", rec.RecId).Updates(map[string]interface{}{"status":rec.Status, "transfer_tx_hash": rec.TransferTx}).Error
								} else {
									err = db.Model(&types.BpRewardRecord{}).Where("id=?", rec.RecId).Update("status",rec.Status).Error
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
		}
	}
	os.Exit(0)
}
