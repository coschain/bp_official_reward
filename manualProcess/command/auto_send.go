package command

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"fmt"
	"github.com/coschain/cobra"
	"github.com/coschain/contentos-go/common/constants"
	"github.com/ethereum/go-ethereum/log"
	"github.com/shopspring/decimal"
	"math/big"
	"os"
)

var (
	AutoSendCmd = func() *cobra.Command {
		cmd := &cobra.Command{
			Use:       "send",
			Short:     "auto send reward to bp or voter of one period",
			Run:       handleAutoSend,
		}
		cmd.Flags().StringVarP(&svEnv, "env", "e", "pro", "service env (default is pro)")
	    cmd.Flags().Uint64VarP(&period,"period",  "p", 0, "period of reward (default is 0)")
		return cmd
	}
)

func handleAutoSend(cmd *cobra.Command, args []string) {
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

	pre := "Auto Send Reward"
	curPeriod,err := db.GetLatestDistributedPeriod(true)
	if err != nil {
		fmt.Printf("%v: fail to get latest period, the error is %v", pre, err)
		os.Exit(1)
	}
	if period == 0 || period > curPeriod {
		period = curPeriod
	}
    recList,err := db.GetAllRewardRecordsOfOnePeriod(period)
    if err != nil {
    	fmt.Printf("fail to get all reward record of period:%v \n", period)
    	os.Exit(1)
	}
    for _,reward := range recList {
    	if reward.Status == types.ProcessingStatusPending && !utils.CheckIsNotEmptyStr(reward.TransferHash) {
    		//transfer reward
    		logger.Infof("%v: transfer vest to account,origin reward record is %+v \n", pre, reward)
    		originAmount := reward.RewardAmount
    		amount,err := decimal.NewFromString(originAmount)
    		if err != nil {
    			logger.Errorf("fail to convert reward amount:%v of record ID:%v, the error is %v", originAmount, reward.Id)
    		} else {
    			amount = amount.Mul(decimal.NewFromFloat(constants.COSTokenDecimals))
				bigAmount := new(big.Int)
				bigAmount.SetString(amount.String(), 10)

				senderAcct,senderPrivkey := config.GetMainNetCosSenderInfo()
				//generate sign tx
				txHash,signedTx,err := utils.GenTransferToVestSignedTx(senderAcct, senderPrivkey, reward.Voter, bigAmount, "")
				if err != nil {
					logger.Errorf("%v: fail to generate transfer vest to %v tx, the error is %v", pre, reward.Voter, err)
					reward.Status = types.ProcessingStatusGenTxFail
					//update reward record on db
					err = db.MdRewardRecord(reward)
					if err != nil {
						logger.Errorf("%v: fail to update rec to %+v", reward)
					}
				} else {
					reward.TransferHash = txHash
					reward.Status = types.ProcessingStatusDefault
					err = db.MdRewardRecord(reward)
					if err != nil {
						logger.Errorf("%v: fail to update rec to %+v", reward)
					} else {
						//send tx
						_,err = utils.TransferVestBySignedTx(signedTx)
						if err != nil {
							logger.Errorf("%v: fail to transfer vest to %v for bp:%v, the error is %v, the rec is %+v", pre, reward.Voter, reward.Bp, err, reward)
						} else {
							logger.Infof("%v: transfer vest to %v for bp:%v", pre, reward.Voter, reward.Bp)
						}
					}

				}

			}
		}
	}
}


