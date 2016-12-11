# Vanadium Core

[![Build Status](https://travis-ci.org/vanadium/core.svg?branch=master)](https://travis-ci.org/vanadium/core)

This is a slimmed down version of Vanadium that is focused on its RPC system.
Building doesn't require the `jiri` tool and everything is in a single git
repository.

## Install steps

Assuming a single path in `$GOPATH`:

```
go get -t v.io/...
VDLPATH=$GOPATH/src go test v.io/...
```

The above will not work if v.io doesn't point to this repo.
