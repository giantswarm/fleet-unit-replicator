PROJECT=fleet-unit-replicator
ifndef registry
registry=registry.giantswarm.io
endif

BUILD_PATH := $(shell pwd)/.gobuild

D0_PATH := $(BUILD_PATH)/src/github.com/giantswarm

BIN := $(PROJECT)

.PHONY: clean run-test get-deps fmt run-tests deps

GOPATH := $(BUILD_PATH)

SOURCE=$(shell find . -name '*.go')
TEMPLATES=$(shell find . -name '*.tmpl')

VERSION=$(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

ifndef GOOS
	GOOS := $(shell go env GOOS)
endif
ifndef GOARCH
	GOARCH := $(shell go env GOARCH)
endif

all: get-deps $(BIN)

ci: clean all run-tests

clean:
	rm -rf $(BUILD_PATH) $(BIN)

get-deps: .gobuild .gobuild/bin/go-bindata

.gobuild/bin/go-bindata:
	GOOS=$(shell go env GOHOSTOS) GOPATH=$(GOPATH) go get github.com/jteeuwen/go-bindata/...

deps:
	@${MAKE} -B -s .gobuild

.gobuild:
	@mkdir -p $(D0_PATH)
	@rm -f $(D0_PATH)/$(PROJECT) && cd "$(D0_PATH)" && ln -s ../../../.. $(PROJECT)
	#
	# Pin versions of certain libs
	@builder get dep -b v0.9.2 git@github.com:coreos/fleet.git $(GOPATH)/src/github.com/coreos/fleet
	#
	# Fetch private packages first (so `go get` skips them later)
	# @builder get dep -b 0.3.0 git@github.com:giantswarm/metrics.git $(D0_PATH)/metrics
	#
	# Fetch pinned external dependencies
	#@builder get dep -b 0.3.0 git@github.com:giantswarm/retry-go.git $(BUILD_PATH)/src/github.com/giantswarm/retry-go
	#
	## Fetch go-etcd compatible with etcd 0.4
	@builder get dep -b release-0.4 git@github.com:coreos/go-etcd.git $(BUILD_PATH)/src/github.com/coreos/go-etcd
	#
	# Fetch public dependencies via `go get`
	# All of the dependencies are listed here to make best use of caching in `builder go get`
	@GOPATH=$(GOPATH) builder go get github.com/ogier/pflag
	@GOPATH=$(GOPATH) builder go get github.com/gorilla/mux
	@GOPATH=$(GOPATH) builder go get github.com/gorilla/context
	@GOPATH=$(GOPATH) builder go get github.com/dchest/uniuri
	@builder get dep git@github.com:docker/docker.git $(BUILD_PATH)/src/github.com/docker/docker
	@GOPATH=$(GOPATH) builder go get github.com/juju/errgo
	@GOPATH=$(GOPATH) builder go get github.com/op/go-logging
	@GOPATH=$(GOPATH) builder go get github.com/pingles/go-metrics-riemann
	@GOPATH=$(GOPATH) builder go get github.com/rcrowley/go-metrics
	# @GOPATH=$(GOPATH) builder go get github.com/inhies/go-tld
	#
	# Build test packages (we only want those two, so we use `-d` in go get)
	GOPATH=$(GOPATH) go get -d -v github.com/onsi/gomega
	GOPATH=$(GOPATH) go get -d -v github.com/onsi/ginkgo
	GOPATH=$(GOPATH) go get -d -v github.com/giantswarm/httptest-helper

$(BIN): VERSION $(SOURCE)
	@echo Building for $(GOOS)/$(GOARCH)
	docker run \
	    --rm \
	    -v $(shell pwd):/usr/code \
	    -e GOPATH=/usr/code/.gobuild \
	    -e GOOS=$(GOOS) \
	    -e GOARCH=$(GOARCH) \
	    -w /usr/code \
	    golang:1.4.2-cross \
	    go build -a -ldflags "-X main.projectVersion $(VERSION) -X main.projectBuild $(COMMIT)" -o $(BIN)

run-tests: 
	@make run-test test=./...

run-test:
	@if test "$(test)" = "" ; then \
		echo "missing test parameter, that is, path to test folder e.g. './middleware/'."; \
		exit 1; \
	fi
	docker run \
	    --rm \
	    -v $(shell pwd):/usr/code \
	    -e GOPATH=/usr/code/.gobuild \
	    -w /usr/code \
	    golang:1.4.2-cross \
	    go test -v $(test)


fmt:
	gofmt -l -w .
