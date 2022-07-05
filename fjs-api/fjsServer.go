package fjs_api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meshplus/pier/cmd/pier/client"
	"github.com/meshplus/pier/internal/repo"
	"github.com/sirupsen/logrus"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const (
	CoWork = 2
)

type FjsServer struct {
	router *gin.Engine
	config *repo.Config
	logger logrus.FieldLogger

	db     *sqlx.DB
	ctx    context.Context
	cancel context.CancelFunc
}

type FJSResponse struct {
	CrsChn       []*CrsChn `json:"CrsChn"`
	CoWorkChains int64     `json:"CoWorkChains"`
	CoWorkSys    int64     `json:"CoWorkSys"`
	CoWorkTrans  int64     `json:"CoWorkTrans"`
	ConnectGWs   int64     `json:"ConnectGWs"`
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
	g.db, err = sqlx.Open("sqlite3", "./fjs.db")
	if err != nil {
		fmt.Printf("sql open filed:%s", err.Error())
		return err
	}
	err = g.createDB(g.db)
	if err != nil {
		return err
	}
	// 定时查询ibtp数据信息
	g.router.Use(gin.Recovery())
	v1 := g.router.Group("/v1")
	{
		// 法监司跨链监控平台接口服务
		v1.GET(client.AnalysisForFJS, g.analysisForFJS)
		//跨链网关处理的跨链协同事务数量，数据类型为int32 - ibtp-id数量为准

	}

	//定时任务每五分钟统计一次数据
	f := func() {
		startDate := time.Now().Add(-time.Minute * 5).Unix()
		endDate := time.Now().Unix()
		timeLayout := "2006-01-02 15:04:05"
		startTime := time.Unix(startDate, 0).Format(timeLayout) //Convert timestamp to standard time
		endTime := time.Unix(endDate, 0).Format(timeLayout)
		CrsChnTxReq1, err := g.db.Queryx("SELECT ibtpid, created from ibtp where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return
		}
		CrsChnTxFail1, err := g.db.Queryx("SELECT ibtpid, created from ibtp_crsChnTxFail where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return
		}
		defer CrsChnTxFail1.Close()
		CrsChnTxProc1, err := g.db.Queryx("SELECT ibtpid, created from ibtp_crsChnTxProc where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return
		}
		defer CrsChnTxProc1.Close()
		var reqCount int64
		for CrsChnTxReq1.Next() {
			reqCount++
		}

		var failReqCount int64
		for CrsChnTxFail1.Next() {
			failReqCount++
		}

		var procReqCount int64
		for CrsChnTxProc1.Next() {
			procReqCount++
		}
		created := endTime
		stmt, err := g.db.Prepare("INSERT INTO ibtp_count(amount,created) VALUES (?,?)")
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}
		_, err = stmt.Exec(reqCount, created)
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}

		stmt, err = g.db.Prepare("INSERT INTO ibtp_crsChnTxProc_count(amount,created) VALUES (?,?)")
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}
		_, err = stmt.Exec(procReqCount, created)
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}

		stmt, err = g.db.Prepare("INSERT INTO ibtp_crsChnTxFail_count(amount,created) VALUES (?,?)")
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}
		_, err = stmt.Exec(failReqCount, created)
		if err != nil {
			fmt.Printf("db insert filed:%s", err.Error())
			return
		}
	}
	var ch chan int
	//定时任务
	ticker := time.NewTicker(time.Minute * 5)
	go func() {
		for range ticker.C {
			f()
		}
		ch <- 1
	}()

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

	count, u := g.count(queryStartTsInt, queryEndTsInt)
	res.CoWorkTrans = u
	res.CrsChn = count
	c.JSON(http.StatusOK, res)
}

type CrsChn struct {
	Name  string `json:"name"`
	Value int64  `db:"iptpid" json:"value"`
	Ts    string `db:"created" json:"ts"`
}

func (g *FjsServer) count(startDate, endDate int64) ([]*CrsChn, int64) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Errorf("%v", e)
		}
	}()
	// insert
	if err2 := g.db.Ping(); err2 != nil {
		fmt.Printf("db ping filed:%s", err2.Error())
		g.db.Close()
		g.db, err2 = sqlx.Open("sqlite3", "./fjs.db")
		if err2 != nil {
			fmt.Printf("db open filed:%s", err2.Error())
			return nil, 0
		}
	}

	CoWorkTrans1, err := g.db.Query("SELECT COUNT (1) from ibtp")
	if err != nil {
		fmt.Printf("db query filed:%s", err.Error())
		return nil, 0
	}
	defer CoWorkTrans1.Close()
	var CoWorkTrans int64
	var resp []*CrsChn
	CoWorkTrans1.Next()
	CoWorkTrans1.Scan(&CoWorkTrans)
	if startDate > 0 && endDate > 0 {
		timeLayout := "2006-01-02 15:04:05"
		startTime := time.Unix(startDate/1000, 0).Format(timeLayout) //Convert timestamp to standard time
		endTime := time.Unix(endDate/1000, 0).Format(timeLayout)
		CrsChnTxReq1, err := g.db.Queryx("SELECT ibtpid, created from ibtp where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return nil, CoWorkTrans
		}
		defer CrsChnTxReq1.Close()
		CrsChnTxFail1, err := g.db.Queryx("SELECT ibtpid, created from ibtp_crsChnTxFail where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return nil, CoWorkTrans
		}
		defer CrsChnTxFail1.Close()
		CrsChnTxProc1, err := g.db.Queryx("SELECT ibtpid, created from ibtp_crsChnTxProc where created >= ? and created <= ?", startTime, endTime)
		if err != nil {
			fmt.Printf("db query filed:%s", err.Error())
			return nil, CoWorkTrans
		}
		defer CrsChnTxProc1.Close()
		req := &CrsChn{Name: "CrsChnTxReq", Ts: startTime}
		var reqCount int64
		for CrsChnTxReq1.Next() {
			reqCount++
		}
		req.Value = reqCount
		resp = append(resp, req)

		failReq := &CrsChn{Name: "CrsChnTxFail", Ts: startTime}
		var failReqCount int64
		for CrsChnTxFail1.Next() {
			failReqCount++
		}
		failReq.Value = failReqCount
		resp = append(resp, failReq)

		procReq := &CrsChn{Name: "CrsChnTxProc", Ts: startTime}
		var procReqCount int64
		for CrsChnTxProc1.Next() {
			procReqCount++
		}
		procReq.Value = procReqCount
		resp = append(resp, procReq)

		//for CrsChnTxFail1.Next() {
		//	a := &CrsChn{Name: "CrsChnTxFail"}
		//	err = CrsChnTxFail1.StructScan(a)
		//	resp = append(resp, a)
		//}
		//for CrsChnTxProc1.Next() {
		//	a := &CrsChn{Name: "CrsChnTxProc"}
		//	err = CrsChnTxProc1.StructScan(a)
		//	resp = append(resp, a)
		//}
	}
	//fmt.Println("resp", resp)
	//fmt.Println("CoWorkTrans", CoWorkTrans)
	return resp, CoWorkTrans
}

func (g *FjsServer) Stop() error {
	g.cancel()
	g.logger.Infoln("fjs gin service stop")
	return nil
}
