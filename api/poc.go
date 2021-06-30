package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/pier/internal/peermgr"
	peerproto "github.com/meshplus/pier/internal/peermgr/proto"
)

func (g *Server) checkHash(c *gin.Context) {
	pierID := c.Query("pier_id")
	hash := c.Query("key")
	res := &response{}
	if pierID == "" {
		res.Data = []byte("pier_id is empty")
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if hash == "" {
		res.Data = []byte("hash is empty")
		c.JSON(http.StatusBadRequest, res)
		return
	}

	msg := peermgr.Message(peerproto.Message_Check_Hash, true, []byte(hash))
	ackMsg, err := g.peerMgr.Send(pierID, msg)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.handleAckCheckHash(c, ackMsg)
}

func (g *Server) handleAckCheckHash(c *gin.Context, msg *peerproto.Message) {
	res := &response{}
	data := msg.Payload.Data
	chr := &pb.CheckHashResponse{}
	err := chr.Unmarshal(data)
	if err != nil {
		res.Data = []byte(err.Error())
		c.JSON(http.StatusInternalServerError, res)
		return
	}
	g.verifyPoc(c, chr, res, g.config.Poc.Validators)
	c.JSON(http.StatusOK, res)
}

func (g *Server) verifyPoc(c *gin.Context, chr *pb.CheckHashResponse, res *response, validators []string) {
	if chr.Res {
		res.Data = []byte("TRUE")
	}
	res.Data = []byte("FALSE")
}

type TendermintCommits struct {
	CommitAddress string
	Commit        string
}

type Tendermint struct {
	Proposal          string
	Height            string
	Round             string
	TendermintCommits []TendermintCommits
}

type Bft struct {
	Proposal string
	Height   uint64
	Round    uint64
	Commits  map[string]string
}

type Proof struct {
	Tendermint Tendermint
	Bft        Bft `json:"Bft"`
}

type PocCitaHeader struct {
	Timestamp        uint64
	PrevHash         string
	Number           string
	StateRoot        string
	TransactionsRoot string
	ReceiptsRoot     string
	QuotaUsed        string
	Proof            Proof
	Proposer         string
}

type Log struct {
	Removed             bool
	LogIndex            uint64
	TransactionIndex    uint64
	TransactionHash     string
	BlockHash           string
	BlockNumber         uint64
	Address             string
	Data                string
	TransactionLogIndex string
	Topics              []string
}

type TransactionReceipt struct {
	TransactionHash     string
	TransactionIndex    uint64
	BlockHash           string
	BlockNumber         uint64
	CumulativeGasUsed   uint64
	CumulativeQuotaUsed uint64
	GasUsed             uint64
	QuotaUsed           uint64
	ContractAddress     string
	Root                string
	Status              string
	From                string
	To                  string
	Logs                []Log
	LogsBloom           string
	ErrorMessage        string
}
