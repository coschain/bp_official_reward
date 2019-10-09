package utils

import (
	"bp_official_reward/config"
	"bp_official_reward/logs"
	"bp_official_reward/rpc"
	"errors"
	"fmt"
	"github.com/coschain/contentos-go/common"
	"github.com/coschain/contentos-go/prototype"
	"github.com/coschain/contentos-go/rpc/pb"
	"math/big"
)

func GenerateSignedTx(privateKey string,client *rpc.CosRpcClient, ops ...interface{}) (*prototype.SignedTransaction, error) {
	privKey := &prototype.PrivateKeyType{}
	pk, err := prototype.PrivateKeyFromWIF(privateKey)
	if err != nil {
		return nil, err
	}
	privKey = pk

	req := &grpcpb.NonParamsRequest{}
	resp, err := client.GetStatisticsInfo(req)
	if err != nil {
		logger := logs.GetLogger()
		logger.Errorf("GenerateSignedTx: fail to get chain statistics info, the error is %v", err)
		return nil, errors.New("fail to get chain statistics info")
	}
	refBlockPrefix :=  common.TaposRefBlockPrefix(resp.State.Dgpo.HeadBlockId.Hash)
	refBlockNum := common.TaposRefBlockNum(resp.State.Dgpo.HeadBlockNumber)
	tx := &prototype.Transaction{RefBlockNum: refBlockNum, RefBlockPrefix: refBlockPrefix, Expiration: &prototype.TimePointSec{UtcSeconds: resp.State.Dgpo.Time.UtcSeconds + 30}}
	for _, op := range ops {
		tx.AddOperation(op)
	}

	signTx := prototype.SignedTransaction{Trx: tx}

	//sign tx
	res := signTx.Sign(privKey, prototype.ChainId{Value: config.GetCosChainId()})
	signTx.Signature = &prototype.SignatureType{Sig: res}

	if err := signTx.Validate(); err != nil {
		return nil, err
	}

	return &signTx, nil
}

func GenTransferToVestSignedTx(from string, privKey string, to string, amount *big.Int, memo string) (string,*prototype.SignedTransaction,error) {
	logger := logs.GetLogger()
	txHash := ""
	rpcClient,err := rpc.CosRpcPoolInstance().GetRpcClient()
	if err != nil {
		logger.Errorf("GenerateTransferSignedTx: Fail to create rpc client, the error is %v", err)
		return txHash, nil, err
	}
	signedTx,err := genTransferToVestSignedTx(from, privKey, to, amount, memo, rpcClient)
	if err != nil {
		logger.Errorf("GenerateTransferSignedTx: Fail to generate signed tx, the error is %v", err)
		return txHash, nil, err
	}
	trxId, err := signedTx.Id()
	if err != nil {
		logger.Errorf("GenerateTransferSignedTx: fail to get signed tx hash, the error is %v", err)
		return txHash, nil, err
	} else {
		txHash = GenMainnetCOSTransferTxId(fmt.Sprintf("%x", trxId.Hash), 0)
	}
	return txHash,signedTx,nil
}

func genTransferToVestSignedTx(from string, privKey string, to string, iAmount *big.Int, memo string, client *rpc.CosRpcClient) (*prototype.SignedTransaction,error) {
	logger := logs.GetLogger()
	if iAmount == nil {
		desc := "GenerateCosTransferSignedTx: can't transfer empty amount"
		logger.Error(desc)
		return nil, errors.New(desc)
	}
	transferAmount := iAmount.Uint64()
	logger.Infof("GenerateCosTransferSignedTx: transfer cos amount is %v", transferAmount)
	op := &prototype.TransferToVestOperation{
		From:   &prototype.AccountName{Value: from},
		To:     &prototype.AccountName{Value: to},
		Amount: prototype.NewCoin(transferAmount),
		Memo:   memo,
	}

	signedTx,err := GenerateSignedTx(privKey, client, op)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

//
// transfer cos by signed tx on main net
//
func TransferVestBySignedTx(signedTx *prototype.SignedTransaction) (txHash string,err error) {
	logger := logs.GetLogger()
	//create rpc client
	rpcClient,err := rpc.CosRpcPoolInstance().GetRpcClient()
	if err != nil {
		logger.Errorf("TransferVestBySignedTx: Fail to create rpc client, the error is %v", err)
		return
	}
	return transferVestBySignedTx(signedTx, rpcClient, false)

}

func transferVestBySignedTx(signedTx *prototype.SignedTransaction, rpcClient *rpc.CosRpcClient,isFinal bool) (string, error){
	txHash := ""
	if signedTx == nil {
		return "", errors.New("transferVestBySignedTx: can't transfer cos with empty tx")
	}

	req := &grpcpb.BroadcastTrxRequest{
		Transaction: signedTx,
		OnlyDeliver: false,
		Finality: isFinal,
	}
	res,err := rpcClient.BroadcastTrx(req)
	if err != nil {
		return txHash, err
	}
	logger := logs.GetLogger()
	if res.Invoice.Status == prototype.StatusSuccess {
		trxId, err := signedTx.Id()
		if err != nil {
			logger.Errorf("transferVestBySignedTx: fail to get signed tx hash, the error is %v", err)
		} else {
			txHash = fmt.Sprintf("%x", trxId.Hash)
		}
	} else {
		return "", errors.New(res.Invoice.ErrorInfo)
	}
	return txHash, nil
}