package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/log"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/cmd/pier/client"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/appchain/proto"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/validation"
	"github.com/sirupsen/logrus"
)

var logger = log.NewWithModule("api_service")

type Gin struct {
	router      *gin.Engine
	peerMgr     peermgr.PeerManager
	appchainMgr *appchain.AppchainMgr
	config      *repo.Config
	logger      logrus.FieldLogger

	ctx    context.Context
	cancel context.CancelFunc
}

func NewGin(appchainMgr *appchain.AppchainMgr, peerMgr peermgr.PeerManager, config *repo.Config) (GinService, error) {
	ctx, cancel := context.WithCancel(context.Background())
	router := gin.New()
	return &Gin{
		router:      router,
		appchainMgr: appchainMgr,
		peerMgr:     peerMgr,
		config:      config,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

func (g *Gin) Start() error {
	g.router.Use(gin.Recovery())
	v1 := g.router.Group("/v1")
	{
		v1.POST(client.RegisterAppchainUrl, g.registerAppchain)
		v1.POST(client.UpdateAppchainUrl, g.updateAppchain)
		v1.POST(client.AuditAppchainUrl, g.auditAppchain)
		v1.GET(client.GetAppchainUrl, g.getAppchain)

		v1.POST(client.RegisterRuleUrl, g.registerRule)
	}

	return g.router.Run(fmt.Sprintf(":%d", g.config.Port.Http))
}

func (g *Gin) Stop() error {
	g.cancel()
	g.logger.Infoln("gin service stop")
	return nil
}

func (g *Gin) auditAppchain(c *gin.Context) {
	var res pb.Response
	var approve client.Approve
	if err := c.BindJSON(&approve); err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusBadRequest, res)
		return
	}
	ok, data := g.appchainMgr.Mgr.Audit(approve.Id, approve.IsApproved, approve.Desc)
	res.Data = data
	if !ok {
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (g *Gin) updateAppchain(c *gin.Context) {
	g.sendAppchain(c, proto.AppchainMessage_UPDATE)
}

func (g *Gin) registerAppchain(c *gin.Context) {
	g.sendAppchain(c, proto.AppchainMessage_REGISTER)
}

func (g *Gin) sendAppchain(c *gin.Context, appchainType proto.AppchainMessage_Type) {
	pierId := c.GetString("pier")
	var res pb.Response
	var appchain appchainmgr.Appchain
	if err := c.BindJSON(&appchain); err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusBadRequest, res)
		return
	}
	data, err := json.Marshal(appchain)
	if err != nil {
		g.logger.Errorln(err)
	}
	am := proto.AppchainMessage{
		Type: appchainType,
		Data: data,
	}
	amData, err := am.Marshal()
	if err != nil {
		g.logger.Errorln(err)
		return
	}
	msg := &peerproto.Message{
		Type: peerproto.Message_APPCHAIN,
		Data: amData,
	}
	ackMsg, err := g.peerMgr.Send(pierId, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.handleAckAppchain(c, ackMsg)
}

func (g *Gin) getAppchain(c *gin.Context) {
	var res pb.Response
	pierId := c.GetString("pier")
	id := c.GetString("id")

	appchain := &appchainmgr.Appchain{
		ID: id,
	}
	data, err := json.Marshal(appchain)
	if err != nil {
		g.logger.Errorln(err)
		return
	}
	am := proto.AppchainMessage{
		Type: proto.AppchainMessage_GET,
		Data: data,
	}
	amData, err := am.Marshal()
	if err != nil {
		g.logger.Errorln(err)
		return
	}
	msg := &peerproto.Message{
		Type: peerproto.Message_APPCHAIN,
		Data: amData,
	}

	ackMsg, err := g.peerMgr.Send(pierId, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.handleAckAppchain(c, ackMsg)
}

func (g *Gin) handleAckAppchain(c *gin.Context, msg *peerproto.Message) {
	data := msg.Data
	am := &proto.AppchainMessage{}
	if err := am.Unmarshal(data); err != nil {
		g.logger.Error(err)
		return
	}
	res := pb.Response{
		Data: am.Data,
	}

	if !am.Ok {
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	app := appchainmgr.Appchain{}
	if err := json.Unmarshal(am.Data, &app); err != nil {
		g.logger.Error(err)
		return
	}

	switch am.Type {
	case proto.AppchainMessage_REGISTER:
		res.Data = []byte(fmt.Sprintf("appchain register successfully, id is %s\n", app.ID))
	case proto.AppchainMessage_UPDATE:
		res.Data = []byte(fmt.Sprintf("appchain update successfully, id is %s\n", app.ID))
	case proto.AppchainMessage_GET:
		res.Data = am.Data
	}
	c.JSON(http.StatusOK, res)
}

func (g *Gin) registerRule(c *gin.Context) {
	pierId := c.GetString("pier")
	var res pb.Response
	rule := &validation.Rule{}
	if err := c.BindJSON(&rule); err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusBadRequest, res)
		return
	}
	data, err := json.Marshal(rule)
	if err != nil {
		g.logger.Errorln(err)
		return
	}
	msg := &peerproto.Message{
		Type: peerproto.Message_RULE,
		Data: data,
	}
	ackMsg, err := g.peerMgr.Send(pierId, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.handleAckRule(c, ackMsg)
}

func (g *Gin) handleAckAppchain(c *gin.Context, msg *peerproto.Message) {
	data := msg.Data
	ruleRes := &validation.RuleResponse{}
	if err := json.Unmarshal(data, ruleRes); err != nil {
		g.logger.Error(err)
		return
	}
	res := pb.Response{
		Data: ruleRes.Content,
	}

	if !ruleRes.Ok {
		c.JSON(http.StatusInternalServerError, res)
		return
	}

	c.JSON(http.StatusOK, res)
}
