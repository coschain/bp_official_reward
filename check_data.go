package main

import (
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/logs"
	"fmt"
)

func main()  {
	fmt.Printf("Start check bp's voter data")
	err := config.SetConfigEnv("pro")
	if err != nil {
		fmt.Printf("fail to set env, the error is %v \n", err)
		return
	}
	err = config.LoadRewardConfig("../bp_reward.json")
	if err != nil {
		fmt.Printf("fail to load reward config, the error is %v \n", err)
		return
	}
	_,err = logs.StartLogService()
	if err != nil {
		fmt.Printf("fail to start log service, the error is %v \n", err)
		return
	}
	err = db.StartDbService()
	if err != nil {
		fmt.Printf("fail to start db service, the error is %v \n", err)
		return
	}
	//var (
	//	sBlkNum uint64 = 16934400
	//	eBlkNum uint64 = 17539200
	//	sBlkTime int64 = 1586334917
	//	eBlkTime int64 = 1586939722
	//)
	curPeriod,err := db.GetLatestDistributedPeriod(true)
	if err != nil {
		fmt.Printf("fail to fetch latest period, the error is %v \n", err)
		return
	}
	sBlkNum := config.ServiceStarPeriodBlockNum + config.DistributeInterval*(curPeriod-1)
	eBlkNum := sBlkNum + config.DistributeInterval
	sBlkInfo,err := db.GetBlockLogByNum(sBlkNum)
	if err != nil {
		fmt.Printf("fail to get block log info of block %v", sBlkNum)
		return
	}
	eBlkInfo,err := db.GetBlockLogByNum(eBlkNum)
	if err != nil {
		fmt.Printf("fail to get block log info of block %v", eBlkNum)
		return
	}
	sBlkTime := sBlkInfo.BlockTime.Unix()
	eBlkTime := eBlkInfo.BlockTime.Unix()
	distributeBpList :=config.OfficialBpList
	extraBpList := config.GetAllCanDistributeExtraRewardBpNameList(eBlkNum)
	if extraBpList != nil && len(extraBpList) > 0 {
		distributeBpList = append(distributeBpList, extraBpList...)
	}

	for _,bp := range distributeBpList {
		fmt.Printf("start check bp:%v \n",bp)
		fmt.Println("============check data===========")
		list,curPeriodVoters,err := db.GetAllValidVotersOfBp(bp, sBlkNum, eBlkNum, sBlkTime, eBlkTime)
		if err != nil {
			fmt.Printf("fail to get all valid voters, the error is %v \n", err)
		} else {
			for _,v := range list {
				fmt.Printf("%v %v\n", v.Name, v.Vest)
			}
		}
		fmt.Println("==============actual data==========")
		fmt.Printf("curPeriodVoters number is %v \n", len(curPeriodVoters))
		voterList,err := db.GetAllRewardedVotersOfPeriodByBp(bp, sBlkTime, eBlkTime, sBlkNum, eBlkNum, curPeriodVoters)
		if err != nil {
			fmt.Printf("fail to get actual voters, the error is %v \n", err)
		} else {
			for _,v := range voterList {
				fmt.Printf("%v %v\n", v.Vest, v.Name)
			}


			isAllCompare := true
			if len(list) != len(voterList) {
				isAllCompare = false
				fmt.Printf("voter count is not comapare, check count is %v, actual count is %v \n", len(list), len(voterList))
			} else {
				for  i := 0; i < len(list); i++ {
					if list[i].Name != voterList[i].Name {
						isAllCompare = false
						fmt.Printf("voter not compare, check voter is %v, actual name is %v \n", list[i].Name, voterList[i].Name)
					} else if list[i].Vest != voterList[i].Vest {
						fmt.Printf("voter vest not compare, check voter vest is %v, actual vest is %v \n", list[i].Vest, voterList[i].Vest)
						isAllCompare = false
					}
				}
			}
			if isAllCompare {
				fmt.Printf("%v:all voter compare, check success", bp)
			} else {
				fmt.Printf("%v:not all voter compare, check fail", bp)
			}
		}
	}
}
