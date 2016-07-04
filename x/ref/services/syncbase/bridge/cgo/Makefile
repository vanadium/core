SHELL := /bin/bash -euo pipefail
GOPATH := $(JIRI_ROOT)/release/go

.DELETE_ON_ERROR:

all: build-ios

.PHONY: build-ios
build-ios: $(shell find . -not -path "./dist/*")
	go env GOPATH
	@mkdir -p dist
	jiri go -target=amd64-ios build -buildmode=c-archive -tags ios -o dist/golib_amd64_ios.a v.io/x/ref/services/syncbase/bridge/cgo
	jiri go -target=arm64-ios build -buildmode=c-archive -tags ios -o dist/golib_arm64_ios.a v.io/x/ref/services/syncbase/bridge/cgo
	cp dist/golib_amd64_ios.h dist/golib.h
	cp lib.h dist/

.PHONY: clean
clean:
	rm -rf dist
