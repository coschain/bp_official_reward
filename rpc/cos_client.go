package rpc

import (
	"context"
	"errors"
	"github.com/coschain/contentos-go/rpc/pb"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"bp_official_reward/config"
	"bp_official_reward/logs"
	"sync"
	"time"
)
var (
	once sync.Once
	rpcPool *CosRpcClientPool
)

type CosRpcClient struct {
	nodeAddr string
	rpcClient grpcpb.ApiServiceClient
	timeout  int
	conn *grpc.ClientConn
	valid bool
}

type CosRpcClientPool struct {
	lock sync.Mutex
	clientMap map[string]*CosRpcClient
}

func CosRpcPoolInstance() *CosRpcClientPool {
	logger := logs.GetLogger()
	once.Do(func() {
		if rpcPool == nil {
			rpcPool = &CosRpcClientPool{
				lock: sync.Mutex{},
			}
			addrList,err := config.GetCosRpcAddrList()
			tOut := config.GetRpcTimeout()
			if err == nil {
				rpcPool.clientMap = initRpcClients(addrList, tOut)
			} else {
				logger.Errorf("CosRpcPoolInstance: fail to get rpc address, the error is %v", err)
			}
		}
	})
	return rpcPool
}


func initRpcClients(addrList []string, timeout int) map[string]*CosRpcClient {
	logger := logs.GetLogger()
	clientMap := make(map[string]*CosRpcClient)
	for _,addr := range addrList {
		cl,err :=  createCOSRpcClient(addr, timeout)
		if err != nil {
			logger.Errorf("Fail to create cos rpc client,the error is %v", err)
		}
		clientMap[addr] = cl
	}
	return clientMap
}

func createCOSRpcClient(addr string, timeout int) (*CosRpcClient,error) {
	cl :=  &CosRpcClient{
		nodeAddr: addr,
		timeout: timeout,
		valid: true,
	}
	logger := logs.GetLogger()
	conn, err := grpc.Dial(addr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(2))))
	if err != nil {
		cl.valid = false
		cl.rpcClient = grpcpb.NewApiServiceClient(&grpc.ClientConn{})
		logger.Errorf("createCOSRpcClient:  fail to dial rpc, the error is %v", err)
		return cl,errors.New("fail to dial rpc")
	}
	rpc := grpcpb.NewApiServiceClient(conn)
	cl.rpcClient = rpc
	cl.conn = conn
	return cl,nil
}

func (pool *CosRpcClientPool) GetRpcClient() (*CosRpcClient,error) {
	logger := logs.GetLogger()
	pool.lock.Lock()
	defer pool.lock.Unlock()
	for _,v := range pool.clientMap {
		if v.valid && v.conn.GetState() != connectivity.Shutdown {
			return v,nil
		}
	}
	//connect again
	logger.Info("RPC: connect all rpc client again")
	addrList,err := config.GetCosRpcAddrList()
	if err != nil {
		logger.Errorf("GetRpcClient: fail to get rpc address, the error is %v", err)
		return nil, errors.New("no valid rpc client")
	}
	pool.clientMap = initRpcClients(addrList, config.GetRpcTimeout())
    for _,v := range pool.clientMap {
    	if v.valid && v.conn.GetState() != connectivity.Shutdown{
    		return v,nil
		}
	}
    err = errors.New("no valid rpc client")
    logger.Errorf(err.Error())
    return nil,err
}


func NewCosRpcClient(addrList []string, timeout int) (*CosRpcClient,error) {
	if len(addrList) < 1 {
		return nil, errors.New("no rpc address available")
	}
	rpcAddr := ""
	var (
		conn *grpc.ClientConn
		err error
	)
	for _,addr := range addrList {
		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(2))))
		if err == nil {
			rpcAddr = addr
			break
		}
	}
    if conn == nil {
    	return nil, errors.New("fail to connect rpc")
	}
	rpc := grpcpb.NewApiServiceClient(conn)
	cl :=  &CosRpcClient{
		nodeAddr: rpcAddr,
		timeout: timeout,
		rpcClient: rpc,
		conn: conn,
	}
	return cl,nil
}

func (client *CosRpcClient) BroadcastTrx(req *grpcpb.BroadcastTrxRequest) (*grpcpb.BroadcastTrxResponse, error) {
	return client.rpcClient.BroadcastTrx(context.Background(), req)
}

func (client *CosRpcClient) GetAccountByName(req *grpcpb.GetAccountByNameRequest) (*grpcpb.AccountResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(client.timeout)*time.Minute)
	defer cancel()
	res, err := client.rpcClient.GetAccountByName(ctx, req)
	return res, err
}

func (client *CosRpcClient) GetStatisticsInfo(req *grpcpb.NonParamsRequest) (*grpcpb.GetChainStateResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(client.timeout)*time.Minute)
	defer cancel()
	res, err := client.rpcClient.GetChainState(ctx, req)
	return res, err
}

func (client *CosRpcClient) GetTop21BpList(req *grpcpb.GetBlockProducerListByVoteCountRequest) (*grpcpb.GetBlockProducerListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(client.timeout)*time.Minute)
	defer cancel()
	res, err := client.rpcClient.GetBlockProducerListByVoteCount(ctx, req)
	return res, err
}

func (client *CosRpcClient) CloseConn() {
	logger := logs.GetLogger()
	if client.conn != nil {
		err := client.conn.Close()
		if err != nil {
			logger.Errorf("Fail to close rpc connect, the error is", err)
		}
	}

}