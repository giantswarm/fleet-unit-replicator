package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	fleet "github.com/coreos/fleet/client"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/registry"
	"github.com/golang/glog"
	"github.com/ogier/pflag"

	"github.com/giantswarm/fleet-unit-replicator/replicator"
)

var (
	config = replicator.Config{}

	glogFlags struct {
		logToStderr     string
		alsoLogToStderr string
		verbosity       string
		vmodule         string
		logBacktraceAt  string
	}

	fleetEtcdPeers = pflag.String("fleet-etcd-peers", "http://localhost:4001/", "List of peers for the fleet client (comma separated).")
	dryRun         = pflag.Bool("dry-run", true, "Do not write to fleet.")
)

func init() {
	pflag.DurationVar(&config.TickerTime, "ticker-time", 60*time.Second, "Ticker time.")
	pflag.DurationVar(&config.DeleteTime, "delete-time", 60*time.Minute, "Time before deleting undesired units.")
	pflag.DurationVar(&config.UpdateCooldownTime, "update-cooldown-time", 15*time.Minute, "Time between updates of changed units.")

	pflag.StringVar(&config.MachineTag, "machine-tag", "", "The machine-tag to filter for.")
	pflag.StringVar(&config.UnitTemplate, "unit-template", "", "The template to render for new units.")
	pflag.StringVar(&config.UnitPrefix, "unit-prefix", "", "The prefix for the units to identify.")

	pflag.StringVar(&glogFlags.logToStderr, "logtostderr", "true", "log to standard error instead of files")
	pflag.StringVar(&glogFlags.alsoLogToStderr, "alsologtostderr", "false", "log to standard error as well as files")
	pflag.StringVar(&glogFlags.verbosity, "v", "1", "log level for V logs")
	pflag.StringVar(&glogFlags.vmodule, "vmodule", "", "comma-separated list of pattern=N settings for file-filtered logging")
	pflag.StringVar(&glogFlags.logBacktraceAt, "log_backtrace_at", "", "when logging hits line file:N, emit a stack trace")
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
	client := &fleet.RegistryClient{reg}
	return client
}

func replicatorConfig() replicator.Config {
	if config.UnitTemplate[0] == '@' {
		filepath := config.UnitTemplate[1:]
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			glog.Fatalf("Failed to open template file %s: %v", filepath, err)
		}

		config.UnitTemplate = string(data)
	}
	return config
}
func replicatorDeps() replicator.Dependencies {
	deps := replicator.Dependencies{
		Fleet: fleetAPI(),
	}
	if *dryRun {
		deps.Operator = &replicator.FleetROOperator{deps.Fleet}
	} else {
		deps.Operator = &replicator.FleetRWOperator{deps.Fleet}
	}
	return deps
}
func main() {
	pflag.Parse()

	// flags have to be passed down to the flag package because glog reads from it.
	flag.Set("logtostderr", glogFlags.logToStderr)
	flag.Set("alsologtostderr", glogFlags.alsoLogToStderr)
	flag.Set("v", glogFlags.verbosity)
	flag.Set("vmodule", glogFlags.vmodule)
	flag.Set("log_traceback_at", glogFlags.logBacktraceAt)

	glog.Info("Fleet Unit Scheduler")
	glog.Info("====================")

	repl := replicator.New(replicatorConfig(), replicatorDeps())
	repl.Run()
}
