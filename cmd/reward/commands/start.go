package commands

import (
	"bp_official_reward/check"
	"bp_official_reward/config"
	"bp_official_reward/db"
	"bp_official_reward/distribute"
	"bp_official_reward/logs"
	"bp_official_reward/snapshot"
	"bp_official_reward/webServer"
	"fmt"
	"github.com/coschain/cobra"
	"github.com/ethereum/go-ethereum/log"
	"os"
	"os/signal"
	"syscall"
)

var svEnv string

var StartCmd = func() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "start",
		Short:     "start cos bp reward service",
		Long:      "start cos bp reward service,if has arg 'env',will use it for service env",
		ValidArgs: []string{"env"},
		Run:       startService,
	}
	cmd.Flags().StringVarP(&svEnv, "env", "e", "pro", "service env (default is pro)")

	return cmd
}

func startService(cmd *cobra.Command, args []string)  {
	fmt.Println("start bp reward service")

	err := config.SetConfigEnv(svEnv)
	if err != nil {
		fmt.Printf("StartService:fail to set env")
		os.Exit(1)
	}

	//load config json file
	err = config.LoadRewardConfig("../../../bp_reward.json")
	if err != nil {
		fmt.Printf("StartService:fail to load config file, the error is %v \n", err)
		os.Exit(1)
	}

	logger,err := logs.StartLogService()
	if err != nil {
		log.Error("fail to start log service")
		os.Exit(1)
	}
	logger.Debug("start bp official reward token service")

	//start db service
	err = db.StartDbService()
	if err != nil {
		logger.Error("StartDbService:fail to start db service")
		os.Exit(1)
	}
	defer db.CloseDbService()
	logger.Debugln("Successfully start db service")
	//start snapshot service
	snapshotSv,err := snapshot.NewBpSyncService()
	if err != nil {
		logger.Error(fmt.Sprintf("NewSnapshotSv:fail to create new snapshot service, error is %v", err))
		os.Exit(1)
	}
	snapshotSv.StartSyncService()
	//start every week reward distribute service
	distributeSv := distribute.NewDistributeService()
	err = distributeSv.StartDistributeService()
	if err != nil {
		logger.Errorf("StartDistributeService: fail to start distribute service, the error is %v", err)
		os.Exit(1)
	}
	checkSv := check.NewRewardResultChecker()
	checkSv.StartCheck()
	distribute.StartGiftTicketCheckService()
	defer func() {
		snapshotSv.StopSyncService()
		distributeSv.StopDistributeService()
		checkSv.StopCheck()
		distribute.CloseGiftTicketCheckService()

	}()
	//start http service
	err = webServer.StartServer()
	if err != nil {
		os.Exit(1)
	}
	defer webServer.StopServer()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}