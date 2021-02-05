package loggers

import (
	"testing"

	"github.com/meshplus/pier/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	config := &repo.Config{
		Log: repo.Log{
			Dir:          "logs",
			Filename:     "pier.log",
			ReportCaller: false,
			Level:        "info",
			Module: repo.LogModule{
				AppchainMgr: "info",
				Exchanger:   "info",
				Executor:    "info",
				BxhLite:     "info",
				Monitor:     "info",
				Swarm:       "info",
				RuleMgr:     "info",
				Syncer:      "info",
				PeerMgr:     "info",
				Router:      "info",
				ApiServer:   "info",
			},
		},
	}
	InitializeLogger(config)
	Logger(ApiServer).Info("api_server")
	exchangerLoggerLevel := w.loggers[Exchanger].Logger.Level.String()
	require.Equal(t, config.Log.Module.Exchanger, exchangerLoggerLevel)
}
