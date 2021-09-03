package router

import (
	"github.com/meshplus/bitxhub-model/pb"
)

//go:generate mockgen -destination mock_router/mock_router.go -package mock_router -source router.go
type Router interface {
	// Start starts the router module
	Start() error

	// Stop
	Stop() error

	//Broadcast broadcasts the registered appchain ids to the union network
	Broadcast(id string) error

	//Route sends ibtp to the union pier in target relay chain
	Route(ibtp *pb.IBTP) error

	QueryInterchain(bxhID, serviceID string) (*pb.Interchain, error)

	QueryIBTP(id string, isReq bool) (*pb.IBTP, error)
}
