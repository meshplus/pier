package bxh_adapter

import "github.com/meshplus/pier/internal/adapt"

//go:generate mockgen -destination mock_BxhAdapt/mock_BxhAdapt.go -package mock_bxhAdapt -source bxh_adapter.go
type BxhAdapterI interface {
	adapt.Adapt

	GetBitXHubChainIDs() ([]string, error)

	ID() string
}
