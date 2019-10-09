package utils

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// generate mainet cos transfer tx id
func GenMainnetCOSTransferTxId(txHash string, opIdx int) string {
	return txHash + "_" + strconv.Itoa(opIdx)
}


// get mainet cos transfer tx hash from tx id
func GetTxHashFromCOSTransferTxId(txId string) string {
	txHash := txId
	sepIdx := strings.LastIndex(txId, "_")
	if sepIdx != -1 {
		txHash = txId[:sepIdx]
	}
	return txHash
}

func GenerateId(t time.Time, orgs ...string) string {
	timestamp := t.Unix()
	timeStr := strconv.FormatInt(timestamp, 10)
	str := timeStr + strings.Join(orgs, "_")
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}


func CheckIsNotEmptyStr(str string) bool {
	if str != "" && len(str) > 0 {
		return true
	}
	return false
}

func ConvertTimeToStamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}