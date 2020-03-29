package executor

//go:generate mockgen -destination mock_executor/mock_executor.go -package mock_executor -source interface.go
type Executor interface {
	// Start starts the service of executor
	Start() error

	// Stop stops the service of executor
	Stop() error
}
