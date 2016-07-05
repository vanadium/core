# Vanadium Core

[![Build Status](https://travis-ci.org/razvanm/vanadium-core.svg?branch=master)](https://travis-ci.org/razvanm/vanadium-core)

This is a slimmed down version of Vanadium that is focused on its RPC system.
Building doesn't require the `jiri` tool and everything is on single git
repository.

## Install steps

The following assumes the current directory is the root of a
[Go workspace](https://golang.org/doc/code.html#Workspaces) and that the
`GOPATH` environmental variables includes it.

```
git clone https://github.com/razvanm/vanadium-core.git src/v.io
go get -t v.io/...
VDLPATH=$PWD/src go test v.io/...
```