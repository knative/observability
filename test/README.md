# Test

This directory contains tests and testing docs for `Knative Observability`:

* [Unit tests](#running-unit-tests) currently reside in the codebase alongside the code they test
* [End-to-end tests](#running-end-to-end-tests), of which there are two types:

## Running unit tests

To run all unit tests:

```bash
go test ./...
```

_By default `go test` will not run [the e2e tests](#running-end-to-end-tests), which need [`-tags=e2e`](#running-end-to-end-tests) to be enabled._

## Running End to End Tests

### Environment Setup

Setting up a running `Knative Observability` cluster.

1. A Kubernetes cluster v1.10 or newer with the `ValidatingAdmissionWebhook`
   admission controller enabled. `kubectl` v1.10 is also required.

1. Deploy Knative Observability components as mentioned in [Deploying Sink
   Resources][deploying]


### Go e2e tests

Once the environment has been setup and the `kubectl` targeting the correct
cluster, run the Go E2E tests with the build tag.

```bash
go test -v -tags=e2e -count=1 -race ./test/e2e/...
```

`-count=1` is the idiomatic way to bypass test caching, so that tests will always run.

### YAML e2e tests

These tests asserts the validation logic for applying the various sink CRDs.

```bash
./test/crd/test.sh
```

### One test case

To run one e2e test case, e.g. TestSimpleBuild, use [the `-run` flag with `go
test`][hdr-test-flags]:

```bash
go test -v -tags=e2e -count=1 -race ./test/e2e/... -run=TestSimpleBuild
```

## Developing End to End Tests

The e2e tests are used to test whether the Knative Observability components
are functioning.

The e2e tests **MUST**:

1. Provide frequent output describing what actions they are undertaking,
   especially before performing long running operations.
1. Follow Golang best practices.
   - [Effective Go][effective-go]


[deploying]: ../README.md#deploying-the-sink-resources
[hdr-test-flags]: https://golang.org/cmd/go/#hdr-Testing_flags
[effective-go]: https://golang.org/doc/effective_go.html
