SHELL := /bin/bash -euo pipefail

GOPATH ?= $(shell pwd)
export GOPATH

VDLPATH ?= $(shell pwd)/src
export VDLPATH
vdlgen:
	go run v.io/x/ref/cmd/vdl generate --lang=go v.io/...	

.PHONY: test-integration
test-integration:
	@echo "GOPATH" ${GOPATH}
	@echo "VDLPATH" ${VDLPATH}
	go test -tags travis \
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

