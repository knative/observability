# Knative Observability

This repo contains a set of in-progress resources designed
to be a lightweight, extensible, and easy-to-understand set of
tools for cluster admins and app developers to egress logs and metrics.

## Setup

Clone the repository into your GOPATH

```
go get github.com/knative/observability
```

## Deploying the Sink Resources

The sink resources can be used without Knative, the only pre-requisite is a
cluster and [`ko`][ko] tool. To deploy simply run the following command from
the `observability` repo directory.

```
KO_DOCKER_REPO=gcr.io/<GCP_PROJECT_ID>/<BUCKET> ko apply -Rf config
```

## Using the Log Sink with Knative

Operators who for regulatory or security reasons want to monitor
logs for every workload on the cluster can use the `clusterlogsink`.
App developers will want to create a `logsink` for their namespace. To
define a `logsink`, apply a yaml similar to the example below:

```yaml
apiVersion: observability.knative.dev/v1alpha1
kind: LogSink
metadata:
  name: logspinner
spec:
  type: syslog
  host: example.com
  port: 25954
  enable_tls: true
```

More examples of logsinks, as well as other resources can be found
in the `test/crd/valid` directory.

The `logsinks` and `clusterlogsinks` can be viewed as follows:

```bash
kubectl get logsinks
kubectl get clusterlogsinks
```

## Using the Cluster Metric Sink with Knative

Operators who wish to gather metrics about running pods and containers can use
the `clustermetricsink` resource. The [telegraf kubernetes input
plugin][telegraf-k8s] is always configured and cannot be removed. At least one
output needs to be provided.

The following example routes kubernetes cluster metrics to datadog.

```yaml
apiVersion: observability.knative.dev/v1alpha1
kind: ClusterMetricSink
metadata:
  name: cluster-metric-sink
spec:
  outputs:
  - type: datadog
    apikey: "datadog-apikey"
```

Refer to [Telegraf's documentation][telegraf-docs] for other configurable
inputs and outputs.

The `clustermetricsinks` can be viewed as follows:

```bash
kubectl get clustermetricsinks
```

## Using the Namespaced Metric Sink with Knative

For developers who want to obtain metrics from within their namespace they can
use the `metricsink` resource. The telegraf agent is deployed as a deployment
within the namespace along with a respective configmap.

It can be configured as follows:

```yaml
apiVersion: observability.knative.dev/v1alpha1
kind: MetricSink
metadata:
  name: metric-sink
spec:
  inputs:
  - type: exec
    commands:
    - "echo 5"
    data_format: "value"
    data_type: "integer"
    name_override: "test"
  outputs:
  - type: datadog
    apikey: apikey
```

Refer to [Telegraf's documentation][telegraf-docs] for other configurable
inputs and outputs.

The `metricsinks` can be viewed as follows:

```bash
kubectl get metricsinks
```

## Developer Notes

The validator and cert-generator images have Dockerfiles and will not be
built using the ko command. They can be built in the following way:

```bash
# From the root of the project directory
docker build --tag cert-generator:dev --file cmd/cert-generator/Dockerfile .
docker build --tag validator:dev --file cmd/validator/Dockerfile .
```

 and in development should be built, uploaded, and
changed in the manifest for testing. The telegraf and fluent-bit images are
also external to this repository, the later can be found at
[fluent-bit-out-syslog plugin][out-syslog].

### Run Tests

See the [Test README][test-readme]

## Technologies

The observability project takes advantage of both fluent-bit and telegraf to
egress metrics and logs. Telegraf is required to run the validating webhook
tests, fluent-bit is not required to run any tests. Fluent-bit is run with the
[fluent-bit-out-syslog plugin][out-syslog] to allow for syslog egress for
logs.

[out-syslog]: https://github.com/pivotal-cf/fluent-bit-out-syslog
[ko]: https://github.com/google/ko
[telegraf-k8s]: https://docs.influxdata.com/telegraf/v1.10/plugins/inputs/#kubernetes
[telegraf-docs]: https://docs.influxdata.com/telegraf/v1.10/plugins/
[test-readme]: test/README.md
