package exchanger

import (
	"github.com/meshplus/pier/internal/adapt"
	"github.com/sirupsen/logrus"
)

type Config struct {
	srcAdapt  adapt.Adapt
	destAdapt adapt.Adapt
	logger    logrus.FieldLogger
}

type Option func(*Config)

func WithSrcAdapt(adapt adapt.Adapt) Option {
	return func(config *Config) {
		config.srcAdapt = adapt
	}
}

func WithDestAdapt(adapt adapt.Adapt) Option {
	return func(config *Config) {
		config.destAdapt = adapt
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(config *Config) {
		config.logger = logger
	}
}

func GenerateConfig(opts ...Option) *Config {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	return config
}
