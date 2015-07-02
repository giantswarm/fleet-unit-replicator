package replicator

import (
	"time"

	"github.com/coreos/fleet/client"
)

type Config struct {
	TickerTime time.Duration
	DeleteTime time.Duration

	MachineTag   string
	UnitPrefix   string
	UnitTemplate string
}

type Dependencies struct {
	Fleet client.API
}

type Service struct {
	Config
	Dependencies
}

func New(cfg Config, deps Dependencies) *Service {
	return &Service{cfg, deps}
}

func (s *Service) Run() {

}
