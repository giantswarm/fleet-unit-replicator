package main

import (
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	fleet "github.com/coreos/fleet/client"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/registry"
	"github.com/giantswarm/metrics"
	"github.com/golang/glog"
	"github.com/ogier/pflag"

	"github.com/giantswarm/fleet-unit-replicator/replicator"
)

var (
	config = replicator.Config{}

	metricFlags = metrics.RegisterMetricFlags(pflag.CommandLine)

	glogFlags struct {
		logToStderr     string
		alsoLogToStderr string
		verbosity       string
		vmodule         string
		logBacktraceAt  string
	}

	fleetDriver   = pflag.String("fleet-driver", "http", "The driver to use for connections to fleet. (http, etcd)")
	fleetEndpoint = pflag.String("fleet-peers", "unix:///var/run/fleet.sock", "List of peers for the fleet client (comma separated).")
	dryRun        = pflag.Bool("dry-run", true, "Do not write to fleet.")
)

func init() {
	pflag.DurationVar(&config.TickerTime, "ticker-time", 60*time.Second, "Ticker time.")
	pflag.DurationVar(&config.DeleteTime, "delete-time", 60*time.Minute, "Time before deleting undesired units.")
	pflag.DurationVar(&config.UpdateCooldownTime, "update-cooldown-time", 15*time.Minute, "Time between updates of changed units.")

	pflag.StringVar(&config.MachineTag, "machine-tag", "", "The machine-tag to filter for.")
	pflag.StringVar(&config.UnitTemplate, "unit-template", "", "The template to render for new units. Prefix with @ to load from a file.")
	pflag.StringVar(&config.UnitPrefix, "unit-prefix", "", "The prefix for the units to identify.")

	pflag.StringVar(&glogFlags.logToStderr, "logtostderr", "true", "log to standard error instead of files")
	pflag.StringVar(&glogFlags.alsoLogToStderr, "alsologtostderr", "false", "log to standard error as well as files")
	pflag.StringVarP(&glogFlags.verbosity, "verbose", "v", "1", "log level for V logs")
	pflag.StringVar(&glogFlags.vmodule, "vmodule", "", "comma-separated list of pattern=N settings for file-filtered logging")
	pflag.StringVar(&glogFlags.logBacktraceAt, "log_backtrace_at", "", "when logging hits line file:N, emit a stack trace")
}

func fleetAPI() fleet.API {
	if *fleetEndpoint == "" {
		glog.Fatalln("No --fleet-fleetEndpoint provided.")
	}
	var fleetClient fleet.API

	switch *fleetDriver {
	case "http":
		ep, err := url.Parse(*fleetEndpoint)
		if err != nil {
			glog.Fatal(err)
		}

		var trans http.RoundTripper

		switch ep.Scheme {
		case "unix", "file":
			// This commonly happens if the user misses the leading slash after the scheme.
			// For example, "unix://var/run/fleet.sock" would be parsed as host "var".
			if len(ep.Host) > 0 {
				glog.Fatalf("unable to connect to host %q with scheme %q\n", ep.Host, ep.Scheme)
			}

			// The Path field is only used for dialing and should not be used when
			// building any further HTTP requests.
			sockPath := ep.Path
			ep.Path = ""

			// http.Client doesn't support the schemes "unix" or "file", but it
			// is safe to use "http" as dialFunc ignores it anyway.
			ep.Scheme = "http"

			// The Host field is not used for dialing, but will be exposed in debug logs.
			ep.Host = "domain-sock"

			trans = &http.Transport{
				Dial: func(s, t string) (net.Conn, error) {
					// http.Client does not natively support dialing a unix domain socket, so the
					// dial function must be overridden.
					return net.Dial("unix", sockPath)
				},
			}
		case "http", "https":
			trans = http.DefaultTransport
		default:
			glog.Fatalf("Unknown scheme in fleet fleetEndpoint: %s\n", ep.Scheme)
		}

		c := &http.Client{
			Transport: trans,
		}

		fleetClient, err = fleet.NewHTTPClient(c, *ep)
		if err != nil {
			glog.Fatalf("Failed to create FleetHttpClient: %s\n", err)
		}
	case "etcd":
		// Code vaguely oriented on fleetctls getRegistryClient()
		// https://github.com/coreos/fleet/blob/2e21d3bfd5959a70513c5e0d3c2500dc3c0811cf/fleetctl/fleetctl.go#L312
		timeout := time.Duration(5 * time.Second)
		machines := strings.Split(*fleetEndpoint, ",")

		trans := &http.Transport{}

		eClient, err := etcd.NewClient(machines, trans, timeout)
		if err != nil {
			glog.Fatalln("Failed to build etcd client: " + err.Error())
		}

		reg := registry.NewEtcdRegistry(eClient, registry.DefaultKeyPrefix)
		fleetClient = &fleet.RegistryClient{reg}
	default:
		glog.Fatalf("Unknown fleet driver: %s\n", *fleetDriver)
	}
	glog.Infof("using fleet driver: %s with fleetEndpoint: %s", *fleetDriver, *fleetEndpoint)

	return fleetClient
}

func replicatorConfig() replicator.Config {
	if config.UnitTemplate == "" {
		glog.Fatalln("No --unit-template provided.")
	}
	if config.UnitPrefix == "" {
		glog.Fatalln("No --unit-prefix provided.")
	}
	if config.MachineTag == "" {
		glog.Warningln("No --machine-tag provided.")
	}
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
		Fleet:   fleetAPI(),
		Metrics: metricFlags.NewMetricsCollector(nil),
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
	os.Args = []string{os.Args[0]}
	flag.Parse()

	// flags have to be passed down to the flag package because glog reads from it.
	flag.Set("logtostderr", glogFlags.logToStderr)
	flag.Set("alsologtostderr", glogFlags.alsoLogToStderr)
	flag.Set("v", glogFlags.verbosity)
	flag.Set("vmodule", glogFlags.vmodule)
	flag.Set("log_traceback_at", glogFlags.logBacktraceAt)

	glog.Info("Fleet Unit Scheduler")
	glog.Info("====================")

	repl := replicator.New(replicatorConfig(), replicatorDeps())
	go repl.Serve()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	glog.Info("Received termination signal. Closing ...")
	repl.Stop()
}
