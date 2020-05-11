package exchanger

type IExchanger interface {
	// Start starts the service of exchanger
	Start() error

	// Stop stops the service of exchanger
	Stop() error
}
