# Knative Observability
This repo contains a set of in progress resources designed
to be a lightweight, extensible, and easy to understand set of
tools for cluster admins and app developers to egress logs and metrics.

## Deploying Log Sink and Cluster Log Sink
The `logsink` and `clusterlogsink` resources can be used without
Knative, the only pre-requisite is a cluster and `ko`. To deploy
simply running the following command from within your go path.

```
KO_DOCKER_REPO=gcr.io/<ProjectID> ko apply -Rf config
```

## Using the Log Sink with Knative
Operators who for regulartory or security reasons want to monitor
logs for every wokload on the cluster can use the `clusterlogsink`.
App developers will want to create a `logsink` for their namespace. To
define a logsink, apply a yaml similar to the example below:

```
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

## Using the Cluster Metric Sink with Knative

Operators who wish to monitor metrics for the cluster can use the
`clustermetricsink`. For regular cluster metrics all that is required
is a telegraf output.

```
apiVersion:observability.knative.dev/v1alpha1
kind: ClusterMetricSink
spec:
  outputs:
  - type: datadog
    apikey: "datadog-apikey"
```

## Dev considerations

The validator and cert-generator images have docker files and will not be
built using the ko command, and in development should be built, uploaded, and
changed in the manifest for testing. The telegraf and fluent-bit images are
also external to this repository, the later being found at
[fluent-bit-out-syslog plugin][out-syslog].

## Technologies

The observability project takes advantage of both fluent-bit and telegraf to
egress metrics and logs. Telegraf is required to run the validating webhook
tests, fluent-bit is not required to run any tests. Fluent-bit is run with the
[fluent-bit-out-syslog plugin][out-syslog] to allow for syslog egress for
logs.

[out-syslog]: https://github.com/pivotal-cf/fluent-bit-out-syslog
