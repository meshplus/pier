package appchain_adapter

import "github.com/meshplus/pier/internal/adapt"

//go:generate mockgen -destination mock_appchainAdapt/mock_appchainAdapt.go -package mock_appchainAdapt -source appchain_adapter.go
type AppchainAdapter interface {
	adapt.Adapt

	GetAppchainID() string
}
