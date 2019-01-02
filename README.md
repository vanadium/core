# Vanadium Core

[![Build Status](https://travis-ci.org/vanadium/core.svg?branch=master)](https://travis-ci.org/vanadium/core)

This is a slimmed down version of Vanadium that is focused on its secure RPC system
and service discovery. See the Vanadium [site](v.io) for more details, remembering
that this repository implements RPC, security and naming; it does not provide support
for mobile development.

## Install steps

Assuming a single path in `$GOPATH`:

```
go get -t v.io/...
VDLPATH=$GOPATH/src go test v.io/...
```
