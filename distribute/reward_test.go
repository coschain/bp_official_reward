package distribute

import (
	"bp_official_reward/config"
	"fmt"
	"testing"
)

func TestReward(t *testing.T) {
	config.ServiceStarPeriodBlockNum = 2419200
	config.DistributeInterval = 604800
	for i:=1; i<52*5; i++ {
		total,_ := CalcSingleBlockRewardByPeriod(uint64(i)).Float64()
		cold,_ := CalcSingleBlockRewardOfColdStartByPeriod(uint64(i)).Float64()
		bp := total - cold
		fmt.Printf("%v: %v, %v, %v, %v\n", i, total, cold, bp, cold/total)
	}
}
