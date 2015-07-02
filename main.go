package main

import (
	"fmt"
	"time"

	"github.com/ogier/pflag"

	"github.com/giantswarm/fleet-unit-replicator/replicator"
)

var (
	config = replicator.Config{}
)

func init() {
	pflag.DurationVar(&config.TicketTimer, "ticket-timer", 60*time.Second, "Ticker time.")
	pflag.IntVar(&config.DeleteTime, "delete-time", 60*time.Minute, "Time before deleting undesired units.")

	pflag.StringVar(&config.MachineTag, "machine-tag", "", "The machine-tag to filter for.")
	pflag.StringVar(&config.UnitTemplate, "unit-template", "", "The template to render for new units.")
	pflag.StringVar(&config.UnitPrefix, "unit-prefix", "", "The prefix for the units to identify.")
}

func main() {
	pflag.Parse()

	fmt.Println("Fleet Unit Scheduler")

	repl := replicator.New(config)
	repl.Start()

	select {} // block for now forever
}
