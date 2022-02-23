[![CircleCI](https://circleci.com/gh/vanadium/core.svg?style=svg)](https://circleci.com/gh/vanadium/core)
![GithubActions](https://github.com/cosnicolaou/pbzip2/actions/workflows/macos.yml/badge.svg)

# Vanadium Core

An easy-to-use secure RPC system with flexible service discovery. See the
Vanadium [v.io](https://v.io) for more details, remembering that this repository
implements RPC, security and naming; it does not provide support for mobile
development nor for syncbase as described at [v.io](https://v.io). The tutorials
on 'Client/Server Basics', 'Security' and 'Naming' are all still current whereas
the ones on 'Syncbase' and 'Java' will no longer work directly.

## Developing Using Vanadium Core

Vanadium Core is a ```go``` module named ```v.io``` and hence packages are
imported as ```v.io/...``` (eg. ```v.io/v23/security```) even though the source
code repository is hosted on [github](https://github.com/vanadium/core).

The associated Vanadium Library (```v.io/x/lib```) is available on
[github](https://github.com/vanadium/go.lib).

Either can be used directly as ```go``` modules.

A single environment variable ```VDLPATH``` is required to run the go tests that
use the VDL language and code generation; it should be set to the directory
that contains the ```go.mod``` for Vanadium Core. Note, that this is only
required for running the VDL code generator and that Vanadium applications
do not require VDLPATH for their correct operation.

## Using Vanadium Commands and Services

All Vanadium Core commands and services can be run using regular ```go``` toolchain
commands, eg: ```go run v.io/x/ref/cmd/principal --help```.

## Installation Steps For Contributors

```
git clone https://github.com/vanadium/core.git
export VDLPATH=$(pwd)/core
cd core
make test test-integration
```

Vanadium Core is capable of using optionally using ```openssl``` for various low level
cyrographic operations; this is purely optionaly and is not required for correct or
performant operation. It's primarily intended to demonstrate interoperability and
to take advantage of more optimized implementations on some systems.

To use ```openssl``` a build tag is required (```--tag=openssl```) and the
```PKG_CONFIG_PATH``` environment variable must be set. For example on an
a MacOS system using ```brew```:

```
export PKG_CONFIG_PATH="$(brew --prefix openssl)/lib/pkgconfig"
```

Note that ```openssl``` version 3 and above is required.

There is an integration test for ```openssl``` support: ```make test-openssl-integration```.


