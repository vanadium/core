SHELL := /bin/bash -euo pipefail

VDLPATH ?= $(shell pwd)
export VDLPATH
vdlgen:
	go run v.io/x/ref/cmd/vdl generate --show-warnings=true --lang=go v.io/...
	go generate ./v23/vdl/vdltest

vdlroot:
	export VDLROOT=$$(pwd)/v23/vdlroot ; \
	cd v23/vdlroot && \
	go run v.io/x/ref/cmd/vdl generate --show-warnings=false --lang=go \
		./vdltool \
		./math \
		./time \
		./signature

.PHONY: test
test:
	@echo "VDLPATH" "${VDLPATH}"
	go test ./...

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
	go get cloudeng.io/go/cmd/goannotate
	go run cloudeng.io/go/cmd/goannotate --config=vanadium-code-annotations.yaml --annotation=copyright ./...
	go mod tidy


bootstrapvdl:
	rm -f $$(find . -name '*.vdl.go')
	export VDLROOT=$$(pwd)/v23/vdlroot ; \
	cd v23/vdlroot && \
	go run -tags vdltoolbootstrapping,vdlbootstrapping \
		 v.io/x/ref/cmd/vdl generate --lang=go ./vdltool && \
 	go run -tags vdlbootstrapping v.io/x/ref/cmd/vdl generate --lang=go \
		v.io/v23/vdl && \
	go run v.io/x/ref/cmd/vdl generate --show-warnings=false --lang=go \
		./math \
		./time \
		./signature
	go run v.io/x/ref/cmd/vdl generate --show-warnings=true --lang=go \
		v.io/v23/uniqueid \
		v.io/v23/vtrace \
		v.io/v23/verror
	go generate ./v23/vdl/vdltest
	go run v.io/x/ref/cmd/vdl generate --show-warnings=true --lang=go v.io/...
	git diff --exit-code

