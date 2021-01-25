package single

import (
	"github.com/meshplus/bitxhub-core/agency"
)

type SinglePierMng struct {
	isMain chan bool
}

func init() {
	agency.RegisterPierHAConstructor("single", New)
}

func New(client agency.HAClient, pierID string) agency.PierHA {
	return &SinglePierMng{
		isMain: make(chan bool),
	}
}

func (s *SinglePierMng) Start() error {
	go func() {
		s.isMain <- true
	}()

	return nil
}

func (s *SinglePierMng) Stop() error {
	return nil
}

func (s *SinglePierMng) IsMain() <-chan bool {
	return s.isMain
}
