package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	sink "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/metric"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	ConfigTelegrafError            = "Failed to validate metricsink config"
	ConfigIncludesKubernetesError  = "Kubernetes input plugin added by default in ClusterMetricSink"
	ConfigLogNoTypeError           = "LogSink should have type"
	ConfigLogChangeTypeError       = "Changing sink type invalid"
	ConfigSyslogBadPortError       = "Port for syslog invalid, should be between 1 and 65535"
	ConfigSyslogBadHostError       = "Host for syslog invalid"
	ConfigSyslogInsecureError      = "Insecure syslog sink not allowed"
	ConfigWebhookBadURLError       = "URL for webhook invalid"
	ConfigWebhookInsecureError     = "Insecure webhook not allowed, scheme must be https"
	ConfigMetricNoTypeError        = "Must specify type for each inputs/outputs"
	ConfigMetricNonStringTypeError = "Input/output type must be a string"
	ConfigMetricNoInputError       = "MetricSinks require at least one input"
)

type ServerOpt func(*Server)

type Server struct {
	mu  sync.Mutex
	lis net.Listener
	srv *http.Server

	addr      string
	tlsConfig *tls.Config
}

func NewServer(addr string, options ...ServerOpt) *Server {
	s := &Server{
		addr: addr,
	}

	for _, o := range options {
		o(s)
	}

	return s
}

func WithTLSConfig(tlsConfig *tls.Config) ServerOpt {
	return func(s *Server) {
		s.tlsConfig = tlsConfig
	}
}

func (s *Server) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}
	go s.run()
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.srv.Shutdown(ctx)
	if err != nil {
		return err
	}
	return s.lis.Close()
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lis == nil {
		return ""
	}

	return s.lis.Addr().String()
}

func (s *Server) run() {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("Unable to start listener: %s", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metricsink", metricSinkHandler)
	mux.HandleFunc("/logsink", logSinkHandler)

	s.mu.Lock()
	s.lis = lis
	s.srv = &http.Server{
		TLSConfig: s.tlsConfig,
		Handler:   mux,
	}
	s.mu.Unlock()

	if s.tlsConfig != nil {
		err = s.srv.ServeTLS(lis, "", "")
	} else {
		err = s.srv.Serve(lis)
	}

	if err != nil {
		log.Printf("Server shutdown: %s", err)
	}
}

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

func healthHandler(_ http.ResponseWriter, _ *http.Request) {}

func metricSinkHandler(w http.ResponseWriter, r *http.Request) {
	requestedAdmissionReview, httpErr := deserializeReview(r)
	if httpErr != nil {
		httpErr.Write(w)
		return
	}

	var cms sink.ClusterMetricSink
	err := json.Unmarshal(requestedAdmissionReview.Request.Object.Raw, &cms)
	if err != nil {
		errUnableToDeserialize.Write(w)
		return
	}

	resp, httpErr := validateMetricSinkConfig(*requestedAdmissionReview, cms)
	if httpErr != nil {
		httpErr.Write(w)
		return
	}

	err = json.NewEncoder(w).Encode(&v1beta1.AdmissionReview{Response: resp})
	if err != nil {
		log.Printf("Unable to marshal resp: %s", err)
	}
}

func toAdmissionErrorResponse(err string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err,
		},
	}
}

func logSinkHandler(w http.ResponseWriter, r *http.Request) {
	requestedAdmissionReview, httpErr := deserializeReview(r)
	if httpErr != nil {
		httpErr.Write(w)
		return
	}
	resp, err := validateLogSinkConfigRequest(requestedAdmissionReview)
	if err != nil {
		errUnableToDeserialize.Write(w)
	}

	err = json.NewEncoder(w).Encode(&v1beta1.AdmissionReview{Response: resp})
	if err != nil {
		log.Printf("Unable to marshal resp: %s", err)
	}
}

func validateLogSinkConfigRequest(rar *v1beta1.AdmissionReview) (*v1beta1.AdmissionResponse, error) {
	var cls sink.ClusterLogSink
	err := json.Unmarshal(rar.Request.Object.Raw, &cls)
	if err != nil {
		return nil, errUnableToDeserialize
	}

	if rar.Request.Operation == "UPDATE" {
		var clsOld sink.ClusterLogSink
		err := json.Unmarshal(rar.Request.OldObject.Raw, &clsOld)
		if err != nil {
			return nil, errUnableToDeserialize
		}
		if clsOld.Spec.Type != cls.Spec.Type {
			return toAdmissionErrorResponse(ConfigLogChangeTypeError), nil
		}
	}

	switch cls.Spec.Type {
	case "syslog":
		if !cls.Spec.EnableTLS {
			return toAdmissionErrorResponse(ConfigSyslogInsecureError), nil
		}
		if cls.Spec.Host == "" {
			return toAdmissionErrorResponse(ConfigSyslogBadHostError), nil
		}
		if cls.Spec.Port > 65535 || cls.Spec.Port < 1 {
			return toAdmissionErrorResponse(ConfigSyslogBadPortError), nil
		}
	case "webhook":
		if cls.Spec.URL == "" {
			return toAdmissionErrorResponse(ConfigWebhookBadURLError), nil
		}
		if !strings.HasPrefix(cls.Spec.URL, "https://") {
			return toAdmissionErrorResponse(ConfigWebhookInsecureError), nil
		}
	default:
		return toAdmissionErrorResponse(ConfigLogNoTypeError), nil
	}
	return &v1beta1.AdmissionResponse{
		UID:     rar.Request.UID,
		Allowed: true,
	}, nil
}

func validRequest(r v1beta1.AdmissionReview) bool {
	return r.Request != nil
}

func validateMetricSinkConfig(rar v1beta1.AdmissionReview, cms sink.ClusterMetricSink) (*v1beta1.AdmissionResponse, *httpError) {
	for _, input := range cms.Spec.Inputs {
		it, ok := input["type"]
		if !ok {
			return toAdmissionErrorResponse(ConfigMetricNoTypeError), nil
		}
		if _, ok = it.(string); !ok {
			return toAdmissionErrorResponse(ConfigMetricNonStringTypeError), nil
		}
		if it == "kubernetes" {
			return toAdmissionErrorResponse(ConfigIncludesKubernetesError), nil
		}
	}
	for _, output := range cms.Spec.Outputs {
		ot, ok := output["type"]
		if !ok {
			return toAdmissionErrorResponse(ConfigMetricNoTypeError), nil
		}
		if _, ok := ot.(string); !ok {
			return toAdmissionErrorResponse(ConfigMetricNonStringTypeError), nil
		}
	}

	if cms.Kind == "MetricSink" && len(cms.Spec.Inputs) == 0 {
		return toAdmissionErrorResponse(ConfigMetricNoInputError), nil
	}

	// Which version of default inputs irrelevant to validation at time of
	// commit.
	cfg := metric.NewConfig("", metric.KubernetesDefault(false))
	cfg.UpsertSink(cms)
	err := ioutil.WriteFile("/tmp/telegraf.conf", []byte(cfg.String()), 0644)
	if err != nil {
		return nil, errUnableToWriteConfig
	}

	cmd := exec.Command("telegraf", "--config", "/tmp/telegraf.conf", "--test")
	err = cmd.Run()
	if err != nil {
		return toAdmissionErrorResponse(ConfigTelegrafError), nil
	}

	return &v1beta1.AdmissionResponse{
		UID:     rar.Request.UID,
		Allowed: true,
	}, nil
}

func deserializeReview(r *http.Request) (*v1beta1.AdmissionReview, *httpError) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, errUnsupportedMedia
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errUnableToReadBody
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("Error closing body: %v\n", err)
		}
	}()

	requestedAdmissionReview := v1beta1.AdmissionReview{}

	deserializer := codecs.UniversalDeserializer()
	_, _, err = deserializer.Decode(data, nil, &requestedAdmissionReview)
	if err != nil {
		return nil, errUnableToDeserialize
	}

	if !validRequest(requestedAdmissionReview) {
		return nil, errInvalidRequest
	}

	return &requestedAdmissionReview, nil
}
