package main

import (
	"fmt"
	fjs_api "github.com/meshplus/pier/fjs-api"
	"github.com/meshplus/pier/internal/loggers"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	fjsServerCMD = cli.Command{
		Name:   "fjsServer",
		Usage:  "Start fjsServer",
		Action: fjsstart,
	}
)

func fjsstart(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}
	repo.SetPath(repoRoot)

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return err
	}
	// init loggers map for pier
	loggers.InitializeLogger(config)
	fjsApiServer, err := fjs_api.NewFjsServer(config, loggers.Logger(loggers.ApiServer))
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	handleShutdownfjs(fjsApiServer, &wg)

	err = fjsApiServer.Start()
	if err != nil {
		return err
	}

	wg.Wait()
	return nil
}

func handleShutdownfjs(pier *fjs_api.FjsServer, wg *sync.WaitGroup) {
	var stop = make(chan os.Signal)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)

	go func() {
		<-stop
		fmt.Println("received interrupt signal, shutting down...")
		if err := pier.Stop(); err != nil {
			logger.Error("pier stop: ", err)
		}

		wg.Done()
		os.Exit(0)
	}()
}
