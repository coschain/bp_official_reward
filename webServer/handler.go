package webServer

import (
	"bp_official_reward/db"
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

)

//
// get a list of user's reward history
//
func getUserRewardHistory(w http.ResponseWriter, r *http.Request)  {
	w.Header().Add("Access-Control-Allow-Origin", "*")

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
				Amount: reward.Amount,
				Time: strconv.FormatInt(reward.Time, 10),
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
		return "", errors.New("empty http request"), http.StatusInternalServerError
	}
	reqMethod := r.Method
	//just handle POST and Get Method
	if reqMethod == http.MethodPost || reqMethod == http.MethodGet {
		if reqMethod == http.MethodGet {
			queryForm, err := url.ParseQuery(r.URL.RawQuery)
			if err == nil && len(queryForm[parameter]) > 0  && utils.CheckIsNotEmptyStr(queryForm[parameter][0]){
				return queryForm[parameter][0], err, http.StatusOK
			} else {
				return "", errors.New(fmt.Sprintf("lack parameter %v", parameter)), types.StatusParamParseError
			}
		} else {
			err = r.ParseForm()
			if err != nil {
				return "", err, types.StatusParamParseError
			}
			val := r.PostFormValue(parameter)
			if len(val) < 1 {
				return "", errors.New(fmt.Sprintf("lack parameter %v", parameter)), types.StatusParamParseError
			}
			return val, nil, http.StatusOK
		}

	} else {
		err = errors.New(fmt.Sprintf("Not support %v method", reqMethod))
		errCode = http.StatusMethodNotAllowed
	}
	return "", err, errCode
}