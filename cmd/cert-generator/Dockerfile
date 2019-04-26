FROM golang:latest as builder

RUN go get -d github.com/cloudflare/cfssl/cmd/cfssl && \
    go get -d github.com/cloudflare/cfssl/cmd/cfssljson && \
    go build -a -installsuffix nocgo -o /cfssl github.com/cloudflare/cfssl/cmd/cfssl && \
    go build -a -installsuffix nocgo -o /cfssljson github.com/cloudflare/cfssl/cmd/cfssljson

FROM ubuntu:xenial

RUN apt-get update && \
    apt-get install --yes curl apt-transport-https && \
    curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" > /etc/apt/sources.list.d/kubernetes.list && \
    apt-get update && \
    apt-get install --yes kubectl jq

COPY --from=builder [ "/cfssl", "/cfssljson", "/usr/local/bin/" ]

RUN groupadd --system cert-gen --gid 1000 && \
    useradd --no-log-init --system --create-home --gid cert-gen cert-gen --uid 1000

USER 1000:1000
WORKDIR /home/cert-gen

COPY [ "cmd/cert-generator/entrypoint.sh", "/usr/local/bin/" ]

CMD [ "entrypoint.sh" ]
