package router

import "github.com/meshplus/bitxhub-model/pb"

type Router interface {
	// Start starts the router module
	Start() error

	// Stop
	Stop() error

	Broadcast(pierIds []string) error

	Route(ibtp *pb.IBTP) error
}
