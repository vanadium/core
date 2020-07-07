SHELL := /bin/bash -euo pipefail

VDLPATH ?= $(shell pwd)
export VDLPATH
vdlgen:
	go run v.io/x/ref/cmd/vdl generate --lang=go v.io/...	

.PHONY: test-integration
test-integration:
	@echo "VDLPATH" ${VDLPATH}
	go test \
		v.io/x/ref/cmd/principal \
		v.io/x/ref/services/identity/identityd \
		v.io/x/ref/services/xproxy/xproxyd \
		v.io/x/ref/services/mounttable/mounttabled \
		v.io/x/ref/services/debug/debug \
		v.io/x/ref/test/hello \
		v.io/x/ref/examples/tunnel/tunneld \
		v.io/x/ref/examples/rps/rpsbot \
		-v23.tests
