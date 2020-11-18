package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/internal/app"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var (
	startCMD = cli.Command{
		Name:   "start",
		Usage:  "Start a long-running daemon process",
		Action: start,
	}
	pluginName = "appchain_plugin"
)

func start(ctx *cli.Context) error {
	fmt.Println(getVersion(true))

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	repo.SetPath(repoRoot)

	config, err := repo.UnmarshalConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("init config error: %s", err)
	}

	err = log.Initialize(
		log.WithReportCaller(config.Log.ReportCaller),
		log.WithPersist(true),
		log.WithFilePath(filepath.Join(repoRoot, config.Log.Dir)),
		log.WithFileName(config.Log.Filename),
		log.WithMaxSize(2*1024*1024),
		log.WithMaxAge(24*time.Hour),
		log.WithRotationTime(24*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("log initialize: %w", err)
	}

	var pier *app.Pier

	switch config.Mode.Type {
	case repo.RelayMode:
		fallthrough
	case repo.DirectMode:
		if err := checkPlugin(); err != nil {
			return fmt.Errorf("check plugin: %w", err)
		}

		pier, err = app.NewPier(repoRoot, config)
		if err != nil {
			return fmt.Errorf("new pier failed: %w", err)
		}
	case repo.UnionMode:
		pier, err = app.NewUnionPier(repoRoot, config)
		if err != nil {
			return fmt.Errorf("new pier failed: %w", err)
		}
	}

	fmt.Printf("Client Type: %s\n", pier.Type())
	runPProf(config.Port.PProf)

	var wg sync.WaitGroup
	wg.Add(1)
	handleShutdown(pier, &wg)

	if err := pier.Start(); err != nil {
		return err
	}

	wg.Wait()

	logger.Info("Pier exits")
	return nil
}

func handleShutdown(pier *app.Pier, wg *sync.WaitGroup) {
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

func runPProf(port int64) {
	go func() {
		addr := fmt.Sprintf("localhost:%d", port)
		fmt.Printf("Pprof on localhost:%d\n\n", port)
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
	}()
}

func checkPlugin() error {
	// check if plugin exists
	pluginRoot, err := repo.PluginPath()
	if err != nil {
		return err
	}

	pluginPath := filepath.Join(pluginRoot, pluginName)
	_, err = os.Stat(pluginPath)
	if err != nil {
		return fmt.Errorf("get plugin file state error: %w", err)
	}

	return nil
}
