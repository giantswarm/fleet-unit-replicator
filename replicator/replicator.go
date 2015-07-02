package replicator

import (
	"time"
)

type Config struct {
	TickerTime time.Duration
	DeleteTime time.Duration

	MachineTag   string
	UnitPrefix   string
	UnitTemplate string
}

type Dependencies struct {
}

type Service struct {
	Config
}

func New(cfg Config) *Service {
	return &Service{cfg}
}

func (s *Service) Start() {

}
