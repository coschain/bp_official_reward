package utils

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/coschain/contentos-go/common/constants"
	"github.com/shopspring/decimal"
	"math/big"
	"strconv"
	"strings"
	"time"
	mRand "math/rand"
	"math"
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
	source := mRand.NewSource(time.Now().UnixNano())
	r := mRand.New(source)
	rStr := strconv.FormatInt(r.Int63n(math.MaxInt64), 10)
	timestamp := t.Unix()
	timeStr := strconv.FormatInt(timestamp, 10)
	str := timeStr + strings.Join(orgs, "_") + rStr
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

func FormatFloatValue(val float64, prec int) string {
	return strconv.FormatFloat(val, 'f', prec, 32)
}

func CalcActualVest(amount uint64) string  {
	bigAmount := new(big.Int).SetUint64(amount)
	d := decimal.NewFromBigInt(bigAmount, 0)
	actual,_ := d.QuoRem(decimal.NewFromFloat(constants.COSTokenDecimals), 6)
	return actual.String()
}