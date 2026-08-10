# Registration Service

[![Go Report Card](https://goreportcard.com/badge/github.com/codeready-toolchain/registration-service)](https://goreportcard.com/report/github.com/codeready-toolchain/registration-service)
[![GoDoc](https://godoc.org/github.com/codeready-toolchain/registration-service?status.png)](https://godoc.org/github.com/codeready-toolchain/registration-service)
[![Codecov.io](https://codecov.io/gh/codeready-toolchain/registration-service/branch/master/graph/badge.svg)](https://codecov.io/gh/codeready-toolchain/registration-service)
[![Operator CD](https://github.com/codeready-toolchain/registration-service/actions/workflows/operator-cd.yml/badge.svg)](https://github.com/codeready-toolchain/registration-service/actions/workflows/operator-cd.yml)

This is the Developer Sandbox Registration Service repository. It implements the registration flow for the Toolchain SaaS.

## Build

Requires Go version 1.26.x (1.26.5 or higher) - download for your development environment [here](https://golang.org/dl/).

This repository uses [Go modules](https://github.com/golang/go/wiki/Modules).

To build, execute:

```
make build
```

This builds the executable with bundled assets. Only the binary needs to be deployed, all static assets are bundled with the binary.

## Development

To make development on the static content easier, use the `./scripts/deploy-dev.sh` shell script with the following commands:

```
$ ./scripts/deploy-dev.sh setup
```
to setup the access to the Container Registry running within the OpenShift cluster accessed via `$KUBECONFIG` and scale down the deployment to 1 replicas

```
$ ./scripts/deploy-dev.sh setup
```
to build the binary, package into an Image, push it to the Container Registry and update the deployment.

### Tests

Tests are run by executing:

```
make test
```

Tests are run with bundled assets, see above.

### VSCode Testing/Debugging

To use the internal test runner and debug features of VSCode, you need to make sure that VSCode runs in a context where Go Modules are enabled. To do this, run:

```
export GO111MODULE=on
```

Before running VSCode from that shell.
