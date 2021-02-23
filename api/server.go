package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/cmd/pier/client"
	"github.com/meshplus/pier/internal/appchain"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
	"github.com/meshplus/pier/internal/repo"
	"github.com/meshplus/pier/internal/rulemgr"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router      *gin.Engine
	peerMgr     peermgr.PeerManager
	appchainMgr *appchain.Manager
	config      *repo.Config
	logger      logrus.FieldLogger

	ctx    context.Context
	cancel context.CancelFunc
}

type response struct {
	Data []byte `json:"data"`
}

func NewServer(appchainMgr *appchain.Manager, peerMgr peermgr.PeerManager, config *repo.Config, logger logrus.FieldLogger) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	return &Server{
		router:      router,
		appchainMgr: appchainMgr,
		peerMgr:     peerMgr,
		config:      config,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

func (g *Server) Start() error {
	g.router.Use(gin.Recovery())
	v1 := g.router.Group("/v1")
	{
		v1.POST(client.RegisterAppchainUrl, g.registerAppchain)
		v1.POST(client.UpdateAppchainUrl, g.updateAppchain)
		v1.POST(client.AuditAppchainUrl, g.auditAppchain)
		v1.GET(client.GetAppchainUrl, g.getAppchain)

		v1.POST(client.RegisterRuleUrl, g.registerRule)
	}

	go func() {
		go func() {
			err := g.router.Run(fmt.Sprintf(":%d", g.config.Port.Http))
			if err != nil {
				panic(err)
			}
		}()
		<-g.ctx.Done()
	}()
	return nil
}

func (g *Server) Stop() error {
	g.cancel()
	g.logger.Infoln("gin service stop")
	return nil
}

func (g *Server) auditAppchain(c *gin.Context) {
	res := &response{}
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

func (g *Server) updateAppchain(c *gin.Context) {
	g.sendAppchain(c, peerproto.Message_APPCHAIN_UPDATE)
}

func (g *Server) registerAppchain(c *gin.Context) {
	g.sendAppchain(c, peerproto.Message_APPCHAIN_REGISTER)
}

func (g *Server) sendAppchain(c *gin.Context, appchainType peerproto.Message_Type) {
	// target pier id
	pierID := c.Query("pier_id")
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

	msg := peermgr.Message(appchainType, true, data)
	ackMsg, err := g.peerMgr.Send(pierID, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}

	g.handleAckAppchain(c, ackMsg)
}

func (g *Server) getAppchain(c *gin.Context) {
	res := &response{}

	// target pier id
	selfPierID, err := g.getSelfPierID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	targetPierID := c.Query("pier_id")
	appchain := &appchainmgr.Appchain{
		ID: selfPierID,
	}

	data, err := json.Marshal(appchain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	msg := peermgr.Message(peerproto.Message_APPCHAIN_GET, true, data)

	ackMsg, err := g.peerMgr.Send(targetPierID, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}

	g.handleAckAppchain(c, ackMsg)
}

func (g *Server) handleAckAppchain(c *gin.Context, msg *peerproto.Message) {
	app := appchainmgr.Appchain{}
	if err := json.Unmarshal(msg.Payload.Data, &app); err != nil {
		g.logger.Error(err)
		return
	}

	res := &response{}

	switch msg.Type {
	case peerproto.Message_APPCHAIN_REGISTER:
		res.Data = []byte(fmt.Sprintf("appchain register successfully, id is %s\n", app.ID))
	case peerproto.Message_APPCHAIN_UPDATE:
		res.Data = []byte(fmt.Sprintf("appchain update successfully, id is %s\n", app.ID))
	case peerproto.Message_APPCHAIN_GET:
		res.Data = msg.Payload.Data
	}

	c.JSON(http.StatusOK, res)
}

func (g *Server) registerRule(c *gin.Context) {
	// target pier id
	pierID := c.Query("pier_id")
	rule := &rulemgr.Rule{}
	res := &response{}
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
	msg := peermgr.Message(peerproto.Message_RULE_DEPLOY, true, data)
	ackMsg, err := g.peerMgr.Send(pierID, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.handleAckRule(c, ackMsg)
}

func (g *Server) handleAckRule(c *gin.Context, msg *peerproto.Message) {
	data := msg.Payload.Data
	ruleRes := &rulemgr.RuleResponse{}
	if err := json.Unmarshal(data, ruleRes); err != nil {
		g.logger.Error(err)
		return
	}
	res := &response{
		Data: []byte(ruleRes.Content),
	}

	if !ruleRes.Ok {
		c.JSON(http.StatusInternalServerError, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (g *Server) getSelfPierID(ctx *gin.Context) (string, error) {
	repoRoot := g.config.RepoRoot
	keyPath := filepath.Join(repoRoot, "key.json")

	privKey, err := asym.RestorePrivateKey(keyPath, "bitxhub")
	if err != nil {
		return "", err
	}

	address, err := privKey.PublicKey().Address()
	if err != nil {
		return "", err
	}

	return address.String(), nil
}
