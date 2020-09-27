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
		coldStartForVoters,_ := CalcSingleBlockRewardOfColdStartByPeriod(uint64(i)).Float64()
		coldStartForBp,_ := CalcSingleBlockRewardOfColdStartForBpByPeriod(uint64(i)).Float64()
		fmt.Printf("%d: %.6f, %.6f, %.3f, %.6f, %.3f\n", i, total, coldStartForVoters, coldStartForVoters/total, coldStartForBp, coldStartForBp/total)
	}
}
