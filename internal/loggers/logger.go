package loggers

import (
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

const (
	App         = "app"
	AppchainMgr = "appchain_mgr"
	RuleMgr     = "rule_mgr"
	BxhLite     = "bxh_lite"
	Exchanger   = "exchanger"
	Monitor     = "monitor"
	Swarm       = "swarm"
	Syncer      = "syncer"
	Executor    = "executor"
)

var w *loggerWrapper

type loggerWrapper struct {
	loggers map[string]*logrus.Entry
}

func InitializeLogger(config *repo.Config) {
	m := make(map[string]*logrus.Entry)
	m[App] = log.NewWithModule(App)
	m[App].Logger.SetLevel(log.ParseLevel(config.Log.Level))
	m[AppchainMgr] = log.NewWithModule(AppchainMgr)
	m[AppchainMgr].Logger.SetLevel(log.ParseLevel(config.Log.Module.AppchainMgr))
	m[RuleMgr] = log.NewWithModule(RuleMgr)
	m[RuleMgr].Logger.SetLevel(log.ParseLevel(config.Log.Module.RuleMgr))
	m[BxhLite] = log.NewWithModule(BxhLite)
	m[BxhLite].Logger.SetLevel(log.ParseLevel(config.Log.Module.BxhLite))
	m[Exchanger] = log.NewWithModule(Exchanger)
	m[Exchanger].Logger.SetLevel(log.ParseLevel(config.Log.Module.Exchanger))
	m[Monitor] = log.NewWithModule(Monitor)
	m[Monitor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Monitor))
	m[Swarm] = log.NewWithModule(Swarm)
	m[Swarm].Logger.SetLevel(log.ParseLevel(config.Log.Module.Swarm))
	m[Syncer] = log.NewWithModule(Syncer)
	m[Syncer].Logger.SetLevel(log.ParseLevel(config.Log.Module.Syncer))
	m[Executor] = log.NewWithModule(Executor)
	m[Executor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Executor))

	w = &loggerWrapper{loggers: m}
}

func Logger(name string) logrus.FieldLogger {
	return w.loggers[name]
}
