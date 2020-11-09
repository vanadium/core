SHELL := /bin/bash -euo pipefail

VDLPATH ?= $(shell pwd)
export VDLPATH
vdlgen:
	go run v.io/x/ref/cmd/vdl generate --errors-no-i18n=false --show-warnings=false --lang=go v.io/...	
	go generate ./v23/vdl/vdltest

VDLROOT ?= $(shell pwd)/v23/vdlroot
export VDLROOT
vdlroot:
	cd v23/vdlroot && \
	go run v.io/x/ref/cmd/vdl generate --errors-no-i18n=false --show-warnings=false --lang=go \
		./vdltool \
		./math \
		./time \
		./signature

.PHONY: test-integration
test-integration:
	@echo "VDLPATH" "${VDLPATH}"
	go test \
		v.io/x/ref/cmd/principal \
		v.io/x/ref/runtime/internal \
		v.io/x/ref/services/identity/identityd \
		v.io/x/ref/services/xproxy/xproxyd \
		v.io/x/ref/services/mounttable/mounttabled \
		v.io/x/ref/services/debug/debug \
		v.io/x/ref/test/hello \
		v.io/x/ref/examples/tunnel/tunneld \
		v.io/x/ref/examples/rps/rpsbot \
		-v23.tests
	go test v.io/v23/security/... -tags openssl
	go test v.io/x/ref/lib/security/... -tags openssl


refresh:
	go generate ./...
	go run cloudeng.io/go/cmd/goannotate --config=vanadium-code-annotations.yaml --annotation=copyright ./...
	go mod tidy

