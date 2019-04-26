FROM golang:1.11 as builder

WORKDIR /root

ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

COPY / /root/

RUN go build \
    -a \
    -installsuffix nocgo \
    -o /validator \
    -mod=readonly \
    -mod=vendor \
    cmd/validator/main.go

FROM ubuntu:xenial

ENV TELEGRAF_VERSION 1.9.3
RUN apt update && apt install -y ca-certificates && update-ca-certificates
ADD https://dl.influxdata.com/telegraf/releases/telegraf_${TELEGRAF_VERSION}-1_amd64.deb /tmp/telegraf_${TELEGRAF_VERSION}-1_amd64.deb

RUN apt install /tmp/telegraf_${TELEGRAF_VERSION}-1_amd64.deb && \
    rm /tmp/telegraf_${TELEGRAF_VERSION}-1_amd64.deb

RUN groupadd --system validator --gid 1000 && \
    useradd --no-log-init --system --gid validator validator --uid 1000

USER 1000:1000

COPY --from=builder /validator /srv/
WORKDIR /srv
CMD [ "/srv/validator" ]
