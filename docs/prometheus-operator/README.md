# Prometheus operator

The purpose of this document is to explain how to setup [Prometheus operator](https://github.com/coreos/prometheus-operator) for Knative observability. It also describes how to scrape metrics endpoints using service monitors.

## Cluster setup

Create a kubernetes cluster as mentioned [here](https://knative.dev/docs/install/knative-with-gke/#creating-a-kubernetes-cluster).

> Note: A GKE cluster is used, but any other cluster type can be used as well.

Export the cluster properties:

```bash
export CLUSTER_NAME="YOUR_CLUSTER_NAME" \
export CLUSTER_PROJECT="YOUR_CLUSTER_PROJECT" \
export CLUSTER_ZONE="YOUR_CLUSTER_ZONE"
```

Create a GKE cluster:

```bash
gcloud beta container --project "${CLUSTER_PROJECT}" clusters create ${CLUSTER_NAME} \
  --addons=HorizontalPodAutoscaling,HttpLoadBalancing,Istio \
  --machine-type=n1-standard-4 \
  --cluster-version=latest --zone=${CLUSTER_ZONE} \
  --enable-stackdriver-kubernetes --enable-ip-alias \
  --enable-autoscaling --min-nodes=1 --max-nodes=10 \
  --enable-autorepair \
  --scopes cloud-platform
```

Grant the cluster-admin permissions:

```bash
kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole=cluster-admin \
  --user=$(gcloud config get-value core/account)
```

## Knative setup

### Install Knative [Serving](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#deploy-knative-serving) and [Eventing](https://github.com/knative/eventing/blob/master/DEVELOPMENT.md#starting-eventing-controller)

> Note: Skip the installation of the monitoring components of Knative serving.

### Configure monitoring for Knative Serving and Eventing

```bash
# configure monitoring for Eventing
cat << EOF | kubectl apply -f -
apiVersion: v1
data:
  metrics.backend-destination: prometheus
kind: ConfigMap
metadata:
  labels:
    eventing.knative.dev/release: devel
  name: config-observability
  namespace: knative-eventing
EOF

# configure monitoring for Serving
cat << EOF | kubectl apply -f -
apiVersion: v1
data:
  metrics.backend-destination: prometheus
kind: ConfigMap
metadata:
  labels:
    serving.knative.dev/release: devel
  name: config-observability
  namespace: knative-serving
EOF
```

## Prometheus operator setup

### Create the `knative-monitoring` namespace

```bash
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: knative-monitoring
  labels:
    monitoring.knative.dev/release: devel
EOF
```

### Install the Prometheus operator

```bash
cat << EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
    app.kubernetes.io/version: v0.33.0
  name: prometheus-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus-operator
subjects:
- kind: ServiceAccount
  name: prometheus-operator
  namespace: knative-monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
    app.kubernetes.io/version: v0.33.0
  name: prometheus-operator
rules:
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - '*'
- apiGroups:
  - monitoring.coreos.com
  resources:
  - alertmanagers
  - prometheuses
  - prometheuses/finalizers
  - alertmanagers/finalizers
  - servicemonitors
  - podmonitors
  - prometheusrules
  verbs:
  - '*'
- apiGroups:
  - apps
  resources:
  - statefulsets
  verbs:
  - '*'
- apiGroups:
  - '*'
  resources:
  - configmaps
  - secrets
  verbs:
  - '*'
- apiGroups:
  - '*'
  resources:
  - pods
  verbs:
  - '*'
- apiGroups:
  - '*'
  resources:
  - services
  - services/finalizers
  - endpoints
  verbs:
  - '*'
- apiGroups:
  - '*'
  resources:
  - nodes
  verbs:
  - '*'
- apiGroups:
  - '*'
  resources:
  - namespaces
  verbs:
  - '*'
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
    app.kubernetes.io/version: v0.33.0
  name: prometheus-operator
  namespace: knative-monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: controller
      app.kubernetes.io/name: prometheus-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/component: controller
        app.kubernetes.io/name: prometheus-operator
        app.kubernetes.io/version: v0.33.0
    spec:
      containers:
      - args:
        - --kubelet-service=kube-system/kubelet
        - --logtostderr=true
        - --config-reloader-image=quay.io/coreos/configmap-reload:v0.0.1
        - --prometheus-config-reloader=quay.io/coreos/prometheus-config-reloader:v0.33.0
        image: quay.io/coreos/prometheus-operator:v0.33.0
        name: prometheus-operator
        ports:
        - containerPort: 8080
          name: http
        resources:
          limits:
            cpu: 200m
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 100Mi
        securityContext:
          allowPrivilegeEscalation: false
      nodeSelector:
        beta.kubernetes.io/os: linux
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      serviceAccountName: prometheus-operator
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
    app.kubernetes.io/version: v0.33.0
  name: prometheus-operator
  namespace: knative-monitoring
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
    app.kubernetes.io/version: v0.33.0
  name: prometheus-operator
  namespace: knative-monitoring
spec:
  clusterIP: None
  ports:
  - name: http
    port: 8080
    targetPort: http
  selector:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: prometheus-operator
EOF
```

### Wait until the Prometheus operator is up and running

```bash
kubectl get pods --namespace knative-monitoring --selector='app.kubernetes.io/component=controller,app.kubernetes.io/name=prometheus-operator' --watch
```

### Create an instance from the Prometheus CR

```bash
cat << EOF | kubectl apply -f -
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  labels:
    app: prometheus
    prometheus: monitoring
  name: monitoring
  namespace: knative-monitoring
spec:
  baseImage: quay.io/prometheus/prometheus
  logLevel: info
  paused: false
  replicas: 1
  resources:
    limits:
      memory: 1Gi
    requests:
      memory: 512Mi
  retention: 2h
  routePrefix: /
  ruleSelector:
    matchLabels:
      prometheus: monitoring
  securityContext:
    fsGroup: 2000
    runAsNonRoot: true
    runAsUser: 1000
  serviceAccountName: prometheus-operator
  serviceMonitorSelector:
    matchLabels:
      prometheus: monitoring
  storage:
    volumeClaimTemplate:
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 4Gi
EOF
```

### Wait until Prometheus is up and running

```bash
kubectl get pod --namespace knative-monitoring prometheus-monitoring-0 --watch
```

## Verification

### NatssChannel setup

Install NATS Streaming server:

```bash
kubectl create namespace natss; \
kubectl apply --namespace natss -f https://raw.githubusercontent.com/knative/eventing/v0.7.0/contrib/natss/config/broker/natss.yaml
```

Install NatssChannel:

```bash
cd ${GOPATH}/src/knative.dev/eventing-contrib/natss/config; \
git checkout master; \
ko apply -f .
```

Add the monitoring port to the **natss-ch-dispatcher** service:

```bash
# edit the natss-ch-dispatcher service
kubectl edit service --namespace knative-eventing natss-ch-dispatcher

# add the following config
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
  - name: metrics-port
    port: 9090
    protocol: TCP
    targetPort: 9090
```

### Create a service monitor for the NatssChannel dispatcher

```bash
cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Service
metadata:
  name: natss-ch-dispatcher-metrics
  namespace: knative-monitoring
  labels:
    contrib.eventing.knative.dev/release: devel
    messaging.knative.dev/channel: natss-channel
    messaging.knative.dev/role: dispatcher
    prometheus: monitoring
spec:
  type: ClusterIP
  ports:
  - name: metrics-port
    port: 9090
  selector:
    messaging.knative.dev/channel: natss-channel
    messaging.knative.dev/role: dispatcher
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: natss-ch-dispatcher-metrics
  namespace: knative-monitoring
  labels:
    contrib.eventing.knative.dev/release: devel
    messaging.knative.dev/channel: natss-channel
    messaging.knative.dev/role: dispatcher
    prometheus: monitoring
spec:
  selector:
    matchLabels:
      messaging.knative.dev/channel: natss-channel
      messaging.knative.dev/role: dispatcher
  endpoints:
  - port: metrics-port
    interval: 10s
  namespaceSelector:
    any: true
EOF
```

### Check Prometheus targets

Do a port-forward to access Prometheus targets endpoint:

```bash
kubectl port-forward --namespace knative-monitoring \
   $(kubectl get pods --namespace knative-monitoring \
   --selector=app=prometheus --output=jsonpath="{.items[0].metadata.name}") \
   9090
```

Access [http://localhost:9090/targets](http://localhost:9090/targets), in a few seconds, the **knative-monitoring/natss-ch-dispatcher-metrics** job should be up.

## Cleanup

```bash
gcloud container clusters delete ${CLUSTER_NAME} --zone ${CLUSTER_ZONE}
```
