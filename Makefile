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
	go install golang.org/x/tools/cmd/goimports@latest
	go test ./...
	cd ./x/ref/examples && go test ./...

.PHONY: test-integration
test-integration:
	@echo "VDLPATH" "${VDLPATH}"
	go test \
		v.io/x/ref/cmd/principal \
		v.io/x/ref/runtime/internal \
		v.io/x/ref/services/xproxy/xproxyd \
		v.io/x/ref/services/mounttable/mounttabled \
		v.io/x/ref/services/debug/debug \
		v.io/x/ref/test/hello \
		-v23.tests
	cd ./x/ref/examples && go test \
		v.io/x/ref/examples/tunnel/tunneld \
		v.io/x/ref/examples/rps/rpsbot \
		--v23.tests
	go test v.io/v23/security/... -tags openssl
	go test v.io/x/ref/lib/security/... -tags openssl

BENCHMARK_MODULES=./v23/security \
	./v23/context \
	./v23/uniqueid \
	./v23/vdl \
	./v23/vom \
	./v23/naming \
	./x/ref/runtime/internal/lib/deque \
	./x/ref/runtime/internal/flow/manager \
	./x/ref/lib/stats/counter \

# Currently this benchmark fails.	
#	./x/ref/runtime/internal/lib/sync

benchmarks:
	for m in $(BENCHMARK_MODULES); do \
		go test -v -run='\^$\' --bench=. $$m; \
	done
	cd ./x/ref/runtime/internal/rpc/benchmark && \
		go test -v -run='\^$\' --bench=.

refresh:
	go generate ./...
	go get cloudeng.io/go/cmd/goannotate
	go run cloudeng.io/go/cmd/goannotate --config=vanadium-code-annotations.yaml --annotation=copyright ./...
	go mod tidy


bootstrapvdl:
	rm -f $$(find . -name '*.vdl.go')
	export VDLROOT=$$(pwd)/v23/vdlroot ; \
	cd v23/vdlroot && \
	echo "bootstrapping vdlroot/vdltool" && \
	go run -tags vdltoolbootstrapping \
		v.io/x/ref/cmd/vdl generate --lang=go ./vdltool && \
	echo "bootstrapping vdlroot/{math,time,signature} and v.io/v23/vdl" && \
	go run -tags vdlbootstrapping \
		v.io/x/ref/cmd/vdl generate --lang=go \
			./math \
			./time \
			./signature \
			v.io/v23/vdl
	echo "generating all other vdl files...."
	go run v.io/x/ref/cmd/vdl generate --lang=go \
			v.io/v23/uniqueid \
			v.io/v23/vtrace \
			v.io/v23/verror
	go generate ./v23/vdl/vdltest
	go run v.io/x/ref/cmd/vdl generate --show-warnings=true --lang=go v.io/...
	git diff --exit-code

