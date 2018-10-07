SHELL := /bin/bash -euo pipefail

GOPATH ?= $(shell pwd)
export GOPATH

VDLPATH ?= $(shell pwd)/src
export VDLPATH

# Note that the split across the core and go.lib repos leads to
# a multi-tree layout when developing for contributing code rather than
# the single tree layout used when 'using' the code via go get for example.
# The VANADIUM_CORE_REPO variable is used to unambigously refer to the
# core repo.
VANADIUM_CORE_REPO ?= $(shell pwd)
export VANADIUM_CORE_REPO


.PHONY: test
test:
	echo "GOPATH" ${GOPATH}
	echo "VDLPATH" ${VDLPATH}
	pwd
	go test v.io/...

.PHONY: test-integration
test-integration:
	@echo "VDLPATH" ${VDLPATH}
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
	@echo "VANADIUM_CORE_REPO" ${VANADIUM_CORE_REPO}
	cd java/lib && ../gradlew -i publishToMavenLocal

.PHONY: test-java
test-java:
	cd java/lib && ../gradlew -i test

.PHONY: clean
clean:
	rm -rf go/bin go/pkg
