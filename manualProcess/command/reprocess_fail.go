package command

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/distribute"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"encoding/json"
	"fmt"
	"github.com/coschain/cobra"
	"io/ioutil"
	"os"
)

var (
	mType int
	ReprocessCmd = func() *cobra.Command {
		cmd := &cobra.Command{
			Use:       "reprocess",
			Short:     "reprocess failed reward records",
			Run:       reprocess,
		}
		cmd.Flags().StringVarP(&svEnv, "env", "e", "pro", "service env (default is pro)")
		cmd.Flags().IntVarP(&mType,"type",  "t", 0, "Reprocess type (default is 0)")

		return cmd
	}
)


func reprocess(cmd *cobra.Command, args []string)  {
	err := config.SetConfigEnv(svEnv)
	if err != nil {
		fmt.Printf("reprocess:fail to set env \n")
		os.Exit(1)
	}

	//load config json file
	err = config.LoadRewardConfig("../../bp_reward.json")
	if err != nil {
		fmt.Println("reprocess:fail to load config file ")
		os.Exit(1)
	}

	logger,err := logs.StartLogService()
	if err != nil {
		fmt.Printf("fail to start log service")
		os.Exit(1)
	}
	//start db service
	err = db.StartDbService()
	if err != nil {
		logger.Error("StartDbService:fail to start db service")
		os.Exit(1)
	}
	defer db.CloseDbService()

	recJson,err := ioutil.ReadFile(config.GetReprocessFilePath())
	if err != nil {
		fmt.Printf("reprocess: fail to load reprocess records, the error is %v \n", err)
		os.Exit(1)
	} else {
		var model types.ManualProcessModel
		err := json.Unmarshal(recJson, &model)
		if err != nil {
			fmt.Printf("reprocess: fail to Unmarshal records, the error is %v \n", err)
			os.Exit(1)
			return
		} else {
			fmt.Printf("model is %+v \n", model)
			distribute.ManualProcessDistribute(&model)
		}
	}
}