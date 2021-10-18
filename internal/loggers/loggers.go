package loggers

import (
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"
)

const (
	ApiServer   = "api_server"
	App         = "app"
	AppchainMgr = "appchain_mgr"
	Appchain    = "appchain"
	BxhLite     = "bxh_lite"
	Executor    = "executor"
	Exchanger   = "exchanger"
	Monitor     = "monitor"
	PeerMgr     = "peer_mgr"
	Router      = "router"
	RuleMgr     = "rule_mgr"
	Swarm       = "swarm"
	Syncer      = "bxh_adapter"
	Direct      = "direct_adapter"
	Union       = "union_adapter"
)

var w *loggerWrapper

type loggerWrapper struct {
	loggers map[string]*logrus.Entry
}

func InitializeLogger(config *repo.Config) {
	m := make(map[string]*logrus.Entry)
	m[ApiServer] = log.NewWithModule(ApiServer)
	m[ApiServer].Logger.SetLevel(log.ParseLevel(config.Log.Module.ApiServer))
	m[App] = log.NewWithModule(App)
	m[App].Logger.SetLevel(log.ParseLevel(config.Log.Level))
	m[AppchainMgr] = log.NewWithModule(AppchainMgr)
	m[AppchainMgr].Logger.SetLevel(log.ParseLevel(config.Log.Module.AppchainMgr))
	m[Appchain] = log.NewWithModule(Appchain)
	m[Appchain].Logger.SetLevel(log.ParseLevel(config.Log.Module.Appchain))
	m[BxhLite] = log.NewWithModule(BxhLite)
	m[BxhLite].Logger.SetLevel(log.ParseLevel(config.Log.Module.BxhLite))
	m[Exchanger] = log.NewWithModule(Exchanger)
	m[Exchanger].Logger.SetLevel(log.ParseLevel(config.Log.Module.Exchanger))
	m[Executor] = log.NewWithModule(Executor)
	m[Executor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Executor))
	m[Monitor] = log.NewWithModule(Monitor)
	m[Monitor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Monitor))
	m[Router] = log.NewWithModule(Router)
	m[Router].Logger.SetLevel(log.ParseLevel(config.Log.Module.Router))
	m[RuleMgr] = log.NewWithModule(RuleMgr)
	m[RuleMgr].Logger.SetLevel(log.ParseLevel(config.Log.Module.RuleMgr))
	m[Swarm] = log.NewWithModule(Swarm)
	m[Swarm].Logger.SetLevel(log.ParseLevel(config.Log.Module.Swarm))
	m[Syncer] = log.NewWithModule(Syncer)
	m[Syncer].Logger.SetLevel(log.ParseLevel(config.Log.Module.Syncer))
	m[PeerMgr] = log.NewWithModule(PeerMgr)
	m[PeerMgr].Logger.SetLevel(log.ParseLevel(config.Log.Module.PeerMgr))
	m[Direct] = log.NewWithModule(Direct)
	m[Direct].Logger.SetLevel(log.ParseLevel(config.Log.Module.Direct))
	m[Union] = log.NewWithModule(Union)
	m[Union].Logger.SetLevel(log.ParseLevel(config.Log.Module.Union))
	w = &loggerWrapper{loggers: m}
}

func Logger(name string) logrus.FieldLogger {
	return w.loggers[name]
}
