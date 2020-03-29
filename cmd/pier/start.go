package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/internal/app"
	"github.com/meshplus/pier/internal/repo"
	"github.com/urfave/cli"
)

var startCMD = cli.Command{
	Name:   "start",
	Usage:  "Start a long-running daemon process",
	Action: start,
}

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

	if err := checkPlugin(config.Appchain.Plugin); err != nil {
		return fmt.Errorf("check plugin: %w", err)
	}

	pier, err := app.NewPier(repoRoot, config)
	if err != nil {
		return err
	}

	fmt.Printf("Client Type: %s\n", pier.Type())
	runPProf(config.Port.PProf)

	if err := pier.Start(); err != nil {
		return err
	}

	c := make(chan struct{})
	<-c

	logger.Info("Pier exits")
	return nil
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

func checkPlugin(name string) error {
	// check if plugin exists
	pluginRoot, err := repo.PluginPath()
	if err != nil {
		return err
	}

	pluginPath := filepath.Join(pluginRoot, name)
	_, err = os.Stat(pluginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin file not exist")
		}

		return fmt.Errorf("get plugin file state error: %w", err)
	}

	return nil
}
