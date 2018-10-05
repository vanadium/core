SHELL := /bin/bash -euo pipefail

GOPATH ?= $(shell pwd)
export GOPATH

VDLPATH := $(shell pwd)/src
export VDLPATH

.PHONY: all
all: go java

.PHONY: go
go: get-deps
	go list v.io/...
	go install v.io/...

.PHONY: get-deps
get-deps: src

src:
	mkdir -p src/v.io
	rsync -a v23 vendor x src/v.io
	git clone https://github.com/vanadium/go.lib src/v.io/x/lib
	go get -t v.io/...

test-all: test test-integration

.PHONY: test
test:
	go test v.io/...

.PHONY: test-integration
test-integration:
	go test \
		v.io/x/ref/cmd/principal \
		v.io/x/ref/services/identity/identityd \
		v.io/x/ref/services/xproxy/xproxyd \
		v.io/x/ref/services/mounttable/mounttabled \
		v.io/x/ref/services/debug/debug \
		v.io/x/ref/services/agent/v23agentd \
		v.io/x/ref/services/agent/vbecome \
		v.io/x/ref/services/agent/agentlib \
		v.io/x/ref/test/hello \
		v.io/x/ref/examples/tunnel/tunneld \
		v.io/x/ref/examples/rps/rpsbot \
		-v23.tests

.PHONY: java
java:
	cd java/lib && ../gradlew -i publishToMavenLocal

.PHONY: test-java
test-java:
	cd java/lib && ../gradlew -i test

.PHONY: clean
clean:
	rm -rf go/bin go/pkg
