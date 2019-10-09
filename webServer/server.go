package webServer

import (
	"bp_official_reward/config"
	"bp_official_reward/logs"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	getRewardHistory = "/api/getRewardHistory"

	writeTimeOut = 2
	readTimeOut  = 2
)

var (
	syncLock sync.Mutex
	server   *http.Server
)


func StartServer() error {
	var g errgroup.Group
	serverMux := initHandlers()
	server = &http.Server{Handler: serverMux, ReadTimeout: readTimeOut * time.Minute, WriteTimeout: writeTimeOut * time.Minute}
	addr := ":" + config.GetHttpPort()
	listener,err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Fail to listen port, the error is %v \n", err)
		return err
	}
	g.Go(func() error {
		fmt.Println("start http server")
		err = server.Serve(listener)
		if err != nil {
			fmt.Printf("Fail to start the http server, the serever is %v \n", err)
		} else {
			fmt.Println("success start http server")
		}
		return err
	})
	err = g.Wait()
	return err
}


func initHandlers() *http.ServeMux {
	serverMux := http.NewServeMux()
	serverMux.HandleFunc(getRewardHistory, func(writer http.ResponseWriter, request *http.Request) {
		getUserRewardHistory(writer, request)
	})
	return serverMux
}

func StopServer()  {
	if err := server.Shutdown(context.Background());err != nil {
		log := logs.GetLogger()
		log.Errorf("StopServer: fail to stop http server, the error is %v", err)
	}
}