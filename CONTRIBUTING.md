

## Updating Dependencies

Make sure the repo exists in your `GOPATH` under:

```
$GOPATH/src/github.com/knative/observability
```

From here run:

```
GO111MODULE=on go mod vendor
git submodule update
```
