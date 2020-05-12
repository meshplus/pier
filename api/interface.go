package api

type GinService interface {
	// Start starts the service of gin
	Start() error

	// Stop stops the service of gin
	Stop() error
}
