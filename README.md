# Vanadium Core

[![CircleCI](https://circleci.com/gh/vanadium/core.svg?style=svg)](https://circleci.com/gh/vanadium/core)

This is a slimmed down version of Vanadium that is focused on its secure RPC system
and service discovery. See the Vanadium [site](v.io) for more details, remembering
that this repository implements RPC, security and naming; it does not provide support
for mobile development.

## Install steps

```
go get -t v.io/...
go test v.io/...
```
