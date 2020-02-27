package command

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"fmt"
	"github.com/coschain/cobra"
	"os"
	"github.com/ethereum/go-ethereum/log"
)

var (
	syncType int //0 sync accounts info; 1:sync vote relations
	fromTableName string
	sTime int64
	eTime int64
	SyncInfoCmd = func() *cobra.Command {
		cmd := &cobra.Command{
			Use:       "sync",
			Short:     "sync accounts info and vote relations data and from one table to another table",
			Run:       handelDataSync,
		}
		cmd.Flags().StringVarP(&svEnv, "env", "e", "dev", "service env (default is pro)")
		cmd.Flags().IntVarP(&syncType,"type",  "p", 0, "sync type (default is 0)")
		cmd.Flags().StringVarP(&fromTableName,"table",  "t", "", "sync from table name")
		cmd.Flags().Int64VarP(&sTime, "start", "b", 0, "begin timestamp")
		cmd.Flags().Int64VarP(&eTime, "finish", "f", 0, "finish timestamp")
		return cmd
	}
)

func handelDataSync(cmd *cobra.Command, args []string)  {
	fmt.Printf("start time is %v, end time %v \n", sTime, eTime)
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

	logger,err := logs.StartLogService()
	if err != nil {
		log.Error("fail to start log service")
		os.Exit(1)
	}
	//start db service
	err = db.StartDbService()
	if err != nil {
		logger.Error("StartDbService:fail to start db service")
		os.Exit(1)
	}
	defer db.CloseDbService()
	if syncType == 0 {
		list,err := db.GetAccountInfoRecordsByRangeTime(fromTableName, sTime, eTime)
		if err != nil {
			logger.Printf("AccountInfoSync:fail to get account info of table:%v, start time is %v, end time is %v, the error is %v\n",fromTableName,sTime, eTime, err)
			return
		}
		err = db.BatchInsertUserVestInfo(list)
		if err != nil {
			logger.Printf("AccountInfoSync: fail to batch insert account info , the error is %v \n", err)
		} else {
			logger.Printf("AccountInfoSync: sync account info successfully")
		}
	} else  if syncType == 1 {
		list,err := db.GetVoteRelationsByRangeTime(fromTableName, sTime, eTime)
		if err != nil {
			logger.Printf("VoteRelationSync:fail to get account info of table:%v, start time is %v, end time is %v, the error is %v \n",fromTableName, sTime, eTime, err)
			return
		}
		err = db.BatchInsertVoteRelation(list)
		if err != nil {
			logger.Printf("VoteRelationSync: fail to batch insert vote relation, the error is %v \n", err)
		} else {
			logger.Printf("VoteRelationSync: sync vote relation successfully")
		}
	}
}