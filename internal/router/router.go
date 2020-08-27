package router

import (
	"github.com/meshplus/bitxhub-model/pb"
	rpcx "github.com/meshplus/go-bitxhub-client"
)

type Router interface {
	// Start starts the router module
	Start() error

	// Stop
	Stop() error

	//Broadcast broadcasts the registered appchain ids to the union network
	Broadcast(ids []string) error

	//Route sends ibtp to the union pier in target relay chain
	Route(ibtp *pb.IBTP) error

	//ExistAppchain returns if appchain id exit in route map
	ExistAppchain(id string) bool

	//AddAppchains adds appchains to route map and broadcast them to union network
	AddAppchains(appchains []*rpcx.Appchain) error
}
