package main

import (
	"crypto/tls"
	"log"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/knative/observability/pkg/webhook"
)

type config struct {
	HTTPAddr string `env:"HTTP_ADDR, required, report"`
	Cert     string `env:"VALIDATOR_CERT, required, report"`
	Key      string `env:"VALIDATOR_KEY, required, report"`
}

func main() {
	cfg := config{
		Cert: "/etc/validator-certs/tls.crt",
		Key:  "/etc/validator-certs/tls.key",
	}
	if err := envstruct.Load(&cfg); err != nil {
		log.Fatalf("Failed to load config from environment: %s", err)
	}

	cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	if err != nil {
		log.Fatalf("Unable to load certs: %s", err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	err = envstruct.WriteReport(&cfg)
	if err != nil {
		log.Printf("Unable to write envstruct report: %s", err)
	}

	webhook.NewServer(cfg.HTTPAddr, webhook.WithTLSConfig(tlsConf)).Run(true)
}
