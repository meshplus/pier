package fjs_api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meshplus/pier/cmd/pier/client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
)

const (
	CoWork = 2
)

type FjsServer struct {
	router *gin.Engine
	config *repo.Config
	logger logrus.FieldLogger

	db     *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
}

type FJSResponse struct {
	CrsChnTxReq  int64 `json:"CrsChnTxReq"`
	CrsChnTxProc int64 `json:"CrsChnTxProc"`
	CrsChnTxFail int64 `json:"CrsChnTxFail"`
	CoWorkChains int64 `json:"CoWorkChains"`
	CoWorkSys    int64 `json:"CoWorkSys"`
	CoWorkTrans  int64 `json:"CoWorkTrans"`
	ConnectGWs   int64 `json:"ConnectGWs"`
}

func NewFjsServer(config *repo.Config, logger logrus.FieldLogger) (*FjsServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	return &FjsServer{
		router: router,
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

type response struct {
	Data []byte `json:"data"`
}

func (g *FjsServer) Start() error {
	var err error
	g.db, err = sql.Open("sqlite3", "./fjs.db")
	if err != nil {
		fmt.Printf("sql open filed:%s", err.Error())
		return err
	}
	err = g.createDB()
	if err != nil {
		return err
	}
	g.router.Use(gin.Recovery())
	v1 := g.router.Group("/v1")
	{
		// 法监司跨链监控平台接口服务
		v1.GET(client.AnalysisForFJS, g.analysisForFJS)
		//跨链网关处理的跨链协同事务数量，数据类型为int32 - ibtp-id数量为准

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

func (g *FjsServer) analysisForFJS(c *gin.Context) {
	res := &FJSResponse{}
	// 经过两个网关，两条应用链
	res.CoWorkChains = CoWork
	res.CoWorkSys = CoWork
	res.ConnectGWs = CoWork
	// 总共的处理ibtp的数量，从exchanger获取

	// 查询开始截止时间，有就返回
	//跨链协同事务实例请求数，数据类型为int32 - 只针对发起链吗 -双边发起
	//完成跨链协同事务实例处理数，数据类型为int32 - 只针对发起链吗 -双边提交
	//跨链协同事务实例处理失败次数，数据类型为int32 - 来自于monitor的事务状态为err的resp请求
	// CrsChnTxReq  CrsChnTxProc  CrsChnTxFail
	//message IndiValue {
	//	string name = 1; //运行状态指标名称
	//	int64 value = 2; //指标的当前状态值
	//	int64 ts = 3; //当前状态值的相应产生时间
	//}
	//		“CoWorkChains”：2
	//		“CoWorkSys”：2
	//		“CoWorkTrans”：跨链网关处理的跨链协同事务数量，数据类型为int32
	//		“ConnectGWs”：2

	queryStartTs := c.DefaultQuery("queryStartTs", "0")
	queryEndTs := c.DefaultQuery("queryEndTs", "0")
	queryStartTsInt, err := strconv.ParseInt(queryStartTs, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	queryEndTsInt, err := strconv.ParseInt(queryEndTs, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if queryStartTsInt > 0 && queryEndTsInt > 0 {
		// 时间范围需要处在14天之内
		if queryEndTsInt < queryStartTsInt || time.Unix(queryEndTsInt/1e3, queryEndTsInt%1e3).After(time.Now()) || time.Unix(queryStartTsInt/1e3, queryStartTsInt%1e3).Before(time.Now().AddDate(0, 0, -14)) {
			c.JSON(http.StatusInternalServerError, "out of date!")
			return
		}
	}
	CoWorkTrans, CrsChnTxReq, CrsChnTxFail, CrsChnTxProc := g.count(queryStartTsInt, queryEndTsInt)
	res.CoWorkTrans = CoWorkTrans
	res.CrsChnTxReq = CrsChnTxReq
	res.CrsChnTxFail = CrsChnTxFail
	res.CrsChnTxProc = CrsChnTxProc
	c.JSON(http.StatusOK, res)
}

func (g *FjsServer) createDB() error {
	//创建表
	sql_table := `
    CREATE TABLE IF NOT EXISTS ibtp(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP
    );
  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxProc(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP
    );
  	CREATE TABLE IF NOT EXISTS ibtp_crsChnTxFail(
        uid INTEGER PRIMARY KEY AUTOINCREMENT,
        ibtpid VARCHAR(64) NULL unique ,
        created TIMESTAMP
    );
    `
	_, err := g.db.Exec(sql_table)
	return err
}

func (g *FjsServer) count(startDate, endDate int64) (int64, int64, int64, int64) {

	defer func() {
		if e := recover(); e != nil {
			fmt.Errorf("%v", e)
		}
	}()
	// insert
	if err2 := g.db.Ping(); err2 != nil {
		fmt.Printf("db ping filed:%s", err2.Error())
		g.db.Close()
		g.db, err2 = sql.Open("sqlite3", "./fjs.db")
		if err2 != nil {
			fmt.Printf("db open filed:%s", err2.Error())
			return 0, 0, 0, 0
		}
	}

	CoWorkTrans1, err := g.db.Query("SELECT COUNT (1) from ibtp")
	if err != nil {
		fmt.Printf("db query filed:%s", err.Error())
		return 0, 0, 0, 0
	}
	defer CoWorkTrans1.Close()
	var CoWorkTrans, CrsChnTxReq, CrsChnTxFail, CrsChnTxProc int64
	CoWorkTrans1.Next()
	CoWorkTrans1.Scan(&CoWorkTrans)
	if startDate > 0 && endDate > 0 {
		CrsChnTxReq1, err := g.db.Query("SELECT COUNT (1) from ibtp where created > ? and created < ?", startDate, endDate)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return 0, 0, 0, 0
		}
		defer CrsChnTxReq1.Close()
		CrsChnTxFail1, err := g.db.Query("SELECT COUNT (1) from ibtp_crsChnTxFail where created > ? and created < ?", startDate, endDate)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return 0, 0, 0, 0
		}
		defer CrsChnTxFail1.Close()
		CrsChnTxProc1, err := g.db.Query("SELECT COUNT (1) from ibtp_crsChnTxProc where created > ? and created < ?", startDate, endDate)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return 0, 0, 0, 0
		}
		defer CrsChnTxProc1.Close()
		CrsChnTxReq1.Next()
		CrsChnTxReq1.Scan(&CrsChnTxReq)
		CrsChnTxFail1.Next()
		CrsChnTxFail1.Scan(&CrsChnTxFail)
		CrsChnTxProc1.Next()
		CrsChnTxProc1.Scan(&CrsChnTxProc)
	}
	return CoWorkTrans, CrsChnTxReq, CrsChnTxFail, CrsChnTxProc
}

func (g *FjsServer) Stop() error {
	g.cancel()
	g.logger.Infoln("fjs gin service stop")
	return nil
}
