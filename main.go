package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	fleet "github.com/coreos/fleet/client"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/registry"
	"github.com/ogier/pflag"

	"github.com/giantswarm/fleet-unit-replicator/replicator"
)

var (
	config = replicator.Config{}

	fleetEtcdPeers = pflag.String("fleet-etcd-peers", "http://localhost:4001/", "List of peers for the fleet client (comma separated).")
)

func init() {
	pflag.DurationVar(&config.TickerTime, "ticker-time", 60*time.Second, "Ticker time.")
	pflag.DurationVar(&config.DeleteTime, "delete-time", 60*time.Minute, "Time before deleting undesired units.")
	pflag.DurationVar(&config.UpdateCooldownTime, "update-cooldown-time", 15*time.Minute, "Time between updates of changed units.")

	pflag.StringVar(&config.MachineTag, "machine-tag", "", "The machine-tag to filter for.")
	pflag.StringVar(&config.UnitTemplate, "unit-template", "", "The template to render for new units.")
	pflag.StringVar(&config.UnitPrefix, "unit-prefix", "", "The prefix for the units to identify.")
}

func fleetAPI() fleet.API {
	// Code vaguely oriented on fleetctls getRegistryClient()
	// https://github.com/coreos/fleet/blob/2e21d3bfd5959a70513c5e0d3c2500dc3c0811cf/fleetctl/fleetctl.go#L312
	timeout := time.Duration(5 * time.Second)
	machines := strings.Split(*fleetEtcdPeers, ",")

	trans := &http.Transport{}

	eClient, err := etcd.NewClient(machines, trans, timeout)
	if err != nil {
		panic("Failed to build etcd client: " + err.Error())
	}

	reg := registry.NewEtcdRegistry(eClient, registry.DefaultKeyPrefix)
	return &fleet.RegistryClient{reg}
}

func main() {
	fmt.Println("Fleet Unit Scheduler")
	fmt.Println("====================")
	pflag.Parse()

	deps := replicator.Dependencies{
		Fleet: fleetAPI(),
	}

	repl := replicator.New(config, deps)
	repl.Run()
}
