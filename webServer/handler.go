package webServer

import (
	"bp_official_reward/db"
	"bp_official_reward/distribute"
	"bp_official_reward/logs"
	"bp_official_reward/types"
	"bp_official_reward/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	pageSizeKey = "pageSize"
	pageIndexKey = "index"
	accountNameKey = "account"
	paramPeriodKey = "period"

	defaultPeriodNum = 4  // default get reward history of the last 4 period
)

//
// get a list of user's reward history
//
func getUserRewardHistory(w http.ResponseWriter, r *http.Request)  {
	w.Header().Add("Access-Control-Allow-Origin", "*")
    logger := logs.GetLogger()
	res := types.UserRewardHistoryResponse{
		List: make([]*types.UserRewardHistory,0),
	}

	//Get user account
	acctName,err,code := parseParameterFromRequest(r, accountNameKey)
	if err != nil {
		res.Status = code
		res.Msg = "params account error"
		writeResponse(w, res)
		return
	}

	//Get page index
	index := types.DefaultPageIndex
	pIndexStr,err,_ := parseParameterFromRequest(r, pageIndexKey)
	if err == nil {
		pIndex,err := strconv.Atoi(pIndexStr)
		if err == nil {
			index = pIndex
		} else {
			logger.Errorf("getUserRewardHistory: fail to convert string %v to int", pIndexStr)
			res.Status = types.StatusParamParseError
			res.Msg = "params index error"
			writeResponse(w, res)
			return
		}
	}

	pageSize := types.DefaultPageSize
	//Get page size param
	pSize,err,_ := parseParameterFromRequest(r, pageSizeKey)
	if err == nil {
		s,err := strconv.Atoi(pSize)
		if err == nil {
			pageSize = s
		} else {
			res.Status = types.StatusParamParseError
			res.Msg = "params pageSize error"
			writeResponse(w, res)
			return
		}
	}

	list,err,code := db.GetUserRewardHistory(acctName, index, pageSize)
	var rewardList []*types.UserRewardHistory

	if err != nil {
		res.Status = code
		res.Msg = err.Error()
	} else {
		res.Status = types.StatusSuccess
		for _,reward := range list {
			history := &types.UserRewardHistory{
				Voter: reward.Voter,
				Bp: reward.Bp,
				Amount: reward.RewardAmount,
				Time: strconv.FormatInt(reward.Time, 10),
				Period: strconv.FormatUint(reward.Period, 10),
				BlockNumber: strconv.FormatUint(reward.DistributeBlockNumber, 10),
			}
			rewardList = append(rewardList, history)
		}
	}

	if rewardList == nil {
		rewardList = make([]*types.UserRewardHistory,0)
	}
	res.List = rewardList
	writeResponse(w, res)
}

//
// get past bp reward records
//
func getBpRewardHistory(w http.ResponseWriter, r *http.Request)  {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	logger := logs.GetLogger()
	res := &types.RewardInfoResponse{
		List: make([]*types.RewardInfo, 0),
	}
	res.Msg = ""
    //get param period
    period := defaultPeriodNum
	periodStr,err,_ := parseParameterFromRequest(r, paramPeriodKey)
	if err == nil {
		p,err := strconv.Atoi(periodStr)
		if err != nil {
			logger.Errorf("getBpRewardHistory: fail to convert string %v to int", periodStr)
			res.Status = types.StatusParamParseError
			writeResponse(w, res)
			return
		}
		period = p
	}
	list,err,code := distribute.GetBpRewardHistory(period)
	if err != nil {
		res.Status = code
		res.Msg = err.Error()
	} else {
		res.Status = types.StatusSuccess
		if list == nil {
			list = make([]*types.RewardInfo, 0)
		}
		res.List = list
	}
	writeResponse(w, res)
}

//
// estimate bp reward of next period
//
func estimateBpRewardOnNextPeriod(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	res := &types.EstimatedRewardInfoResponse{
		Info: &types.EstimatedRewardInfoModel{},
	}
	res.Msg = ""
	info,err,code := distribute.EstimateCurrentPeriodReward()
	if err != nil {
		res.Status = code
		res.Msg = err.Error()
	} else {
		res.Status = types.StatusSuccess
		res.Info = info
	}
	writeResponse(w, res)
	
}

//
// get historical vote info(include total voter number and max ROI)
//
func getHistoricalVoteInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
    res := &types.HistoricalVotingDataResponse{
    	Info: &types.HistoricalVotingData{},
	}
    res.Msg = ""
    info,err,code := distribute.GetHistoricalVotingData()
	if err != nil {
		res.Status = code
		res.Msg = err.Error()
	} else {
		res.Info = info
		res.Status = types.StatusSuccess
	}
	writeResponse(w, res)
}

func writeResponse(w http.ResponseWriter, data interface{}) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Fail to marshal json", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _,err := w.Write(js); err != nil {
		log := logs.GetLogger()
		log.Errorf("w.Write fail, json is %v, error is %v \n", string(js), err)
		http.Error(w, "Fail to write json", types.StatusWriteJsonError)
	}

}

func parseParameterFromRequest(r *http.Request, parameter string) (string,error,int) {
	var (
		err error
		errCode int
	)

	if r == nil {
		return "", errors.New("empty http request"), types.StatusParamParseError
	}
	reqMethod := r.Method
	//just handle POST and Get Method
	if reqMethod == http.MethodPost || reqMethod == http.MethodGet {
		if reqMethod == http.MethodGet {
			queryForm, err := url.ParseQuery(r.URL.RawQuery)
			if err == nil && len(queryForm[parameter]) > 0  && utils.CheckIsNotEmptyStr(queryForm[parameter][0]){
				return queryForm[parameter][0], err, http.StatusOK
			} else {
				return "", errors.New(fmt.Sprintf("lack parameter %v", parameter)), types.StatusLackParamError
			}
		} else {
			err = r.ParseForm()
			if err != nil {
				return "", err, types.StatusParamParseError
			}
			val := r.PostFormValue(parameter)
			if len(val) < 1 {
				return "", errors.New(fmt.Sprintf("lack parameter %v", parameter)), types.StatusLackParamError
			}
			return val, nil, http.StatusOK
		}

	} else {
		err = errors.New(fmt.Sprintf("Not support %v method", reqMethod))
		errCode = http.StatusMethodNotAllowed
	}
	return "", err, errCode
}