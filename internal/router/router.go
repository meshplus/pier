package router

import (
	appchainmgr "github.com/meshplus/bitxhub-core/appchain-mgr"
	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_router/mock_router.go -package mock_router -source router.go
type Router interface {
	// Start starts the router module
	Start() error

	// Stop
	Stop() error

	//Broadcast broadcasts the registered appchain ids to the union network
	Broadcast(ids []string) error

	//Route sends ibtp to the union pier in target relay chain
	Route(ibtp *pb.IBTP) error

	//ExistAppchain returns if appchain id exist in route map
	ExistAppchain(id string) bool

	//AddAppchains adds appchains to route map and broadcast them to union network
	AddAppchains(appchains []*appchainmgr.Appchain) error
}
