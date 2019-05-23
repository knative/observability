package webhook_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/observability/pkg/webhook"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var telegrafExists bool = func() bool {
	err := exec.Command("which", "telegraf").Run()
	return err == nil
}()

func requireTelegraf(t *testing.T) {
	if !telegrafExists {
		t.Skip("telegraf is required to run this test")
	}
}

func init() {
	log.SetOutput(ioutil.Discard)
}

type invalidValidationTest struct {
	name          string
	specObject    string
	errorResponse string
}

type validValidationTest struct {
	name       string
	specObject string
}

func TestValidator(t *testing.T) {
	t.Run("it returns 200 for health endpoint", func(t *testing.T) {
		server := webhook.NewServer("127.0.0.1:0")
		server.Run(false)
		defer server.Close()

		var (
			err  error
			resp *http.Response
		)
		for i := 0; i < 100; i++ {
			resp, err = http.Get(
				"http://" + server.Addr() + "/health",
			)
			if err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected http status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("it expects a content type of application/json", func(t *testing.T) {
		server := webhook.NewServer("127.0.0.1:0")
		server.Run(false)
		defer server.Close()

		for _, endpoint := range []string{"metricsink", "logsink"} {
			t.Run(endpoint, func(t *testing.T) {
				var (
					err  error
					resp *http.Response
				)
				for i := 0; i < 100; i++ {
					resp, err = http.Post(
						fmt.Sprintf("http://%s/%s", server.Addr(), endpoint),
						"text/plain",
						strings.NewReader(`{}`),
					)
					if err == nil {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err != nil {
					t.Error(err)
				}

				if resp.StatusCode != http.StatusUnsupportedMediaType {
					t.Errorf("expected http status 415, got %d", resp.StatusCode)
				}
			})
		}
	})

	t.Run("it returns non-200 response code if unable to deserialize request", func(t *testing.T) {
		server := webhook.NewServer("127.0.0.1:0")
		server.Run(false)
		defer server.Close()

		var (
			err  error
			resp *http.Response
		)
		for _, endpoint := range []string{"metricsink", "logsink"} {
			t.Run(endpoint, func(t *testing.T) {
				for i := 0; i < 100; i++ {
					resp, err = http.Post(
						fmt.Sprintf("http://%s/%s", server.Addr(), endpoint),
						"application/json",
						strings.NewReader(`{ asdasd }`),
					)
					if err == nil {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err != nil {
					t.Error(err)
				}
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("expected http status 400, got %d", resp.StatusCode)
				}
			})
		}
	})

	t.Run("it returns non-200 response code if unable to deserialize object", func(t *testing.T) {
		endpoints := []string{"metricsink", "logsink"}
		server := webhook.NewServer("127.0.0.1:0")
		server.Run(false)
		defer server.Close()

		for _, endpoint := range endpoints {
			t.Run(endpoint, func(t *testing.T) {
				var (
					err  error
					resp *http.Response
				)
				for i := 0; i < 100; i++ {
					resp, err = http.Post(
						fmt.Sprintf("http://%s/%s", server.Addr(), endpoint),
						"application/json",
						strings.NewReader(`{}`),
					)
					if err == nil {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err != nil {
					t.Error(err)
				}
				if resp.StatusCode != http.StatusUnprocessableEntity {
					t.Errorf("expected http status 422, got %d", resp.StatusCode)
				}
			})
		}
	})

	t.Run("LogSink", func(t *testing.T) {
		t.Run("returns an allowed admission response for", func(t *testing.T) {
			tests := []validValidationTest{
				{
					"syslog",
					`{
						"type": "syslog",
						"host": "example.com",
						"port": 100,
						"enable_tls": true
					}`,
				},
				{
					"webhook",
					`{
						"type": "webhook",
						"url": "https://example.com/place"
					}`,
				},
			}
			server := webhook.NewServer("127.0.0.1:0")
			server.Run(false)
			defer server.Close()

			for _, test := range tests {
				for ttype, template := range map[string]string{
					"cluster":   clusterLogSinkAdmissionTemplate,
					"namespace": logSinkAdmissionTemplate,
				} {
					t.Run(test.name+"/"+ttype, func(t *testing.T) {
						var (
							err  error
							resp *http.Response
						)
						for i := 0; i < 100; i++ {
							resp, err = http.Post(
								"http://"+server.Addr()+"/logsink",
								"application/json",
								strings.NewReader(fmt.Sprintf(template, test.specObject)),
							)
							if err == nil {
								break
							}
							time.Sleep(5 * time.Millisecond)
						}
						if err != nil {
							t.Error(err)
						}
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected http status 200, got %d", resp.StatusCode)
						}
						defer resp.Body.Close()

						var actualResp v1beta1.AdmissionReview
						err = json.NewDecoder(resp.Body).Decode(&actualResp)
						if err != nil {
							t.Errorf("unable to decode resp body: %s", err)
						}

						if !actualResp.Response.Allowed {
							t.Errorf("expected response to be allowed, got false")
						}
					})
				}
			}
		})

		t.Run("returns a disallowed admission response for", func(t *testing.T) {
			tests := []invalidValidationTest{
				{
					"no type",
					`{
						"host": "example.com",
						"port": 0,
						"enable_tls": true
					}`,
					"LogSink should have type",
				},
				{
					"high port",
					`{
						"type": "syslog",
						"host": "example.com",
						"port": 100000,
						"enable_tls": true
					}`,
					"Port for syslog invalid, should be between 1 and 65535",
				},
				{
					"low port",
					`{
						"type": "syslog",
						"host": "example.com",
						"port": 0,
						"enable_tls": true
					}`,
					"Port for syslog invalid, should be between 1 and 65535",
				},
				{
					"no port",
					`{
						"type": "syslog",
						"host": "example.com",
						"enable_tls": true
					}`,
					"Port for syslog invalid, should be between 1 and 65535",
				},
				{
					"no host",
					`{
						"type": "syslog",
						"port": 0,
						"enable_tls": true
					}`,
					"Host for syslog invalid",
				},
				{
					"no url",
					`{
						"type": "webhook"
					}`,
					"URL for webhook invalid",
				},
				{
					"mismatch properties",
					`{
						"type": "webhook",
						"host": "example.com",
						"port": 5678
					}`,
					"URL for webhook invalid",
				},
				{
					"insecure syslog",
					`{
						"type": "syslog",
						"host": "example.com",
						"port": 5678
					}`,
					"Insecure syslog sink not allowed",
				},
				{
					"insecure webhook",
					`{
						"type": "webhook",
						"url": "http://webhook.com"
					}`,
					"Insecure webhook not allowed, scheme must be https",
				},
			}
			server := webhook.NewServer("127.0.0.1:0")
			server.Run(false)
			defer server.Close()

			for _, test := range tests {
				for ttype, template := range map[string]string{
					"cluster":   clusterLogSinkAdmissionTemplate,
					"namespace": logSinkAdmissionTemplate,
				} {
					t.Run(test.name+"/"+ttype, func(t *testing.T) {
						var (
							err  error
							resp *http.Response
						)
						for i := 0; i < 100; i++ {
							resp, err = http.Post(
								"http://"+server.Addr()+"/logsink",
								"application/json",
								strings.NewReader(fmt.Sprintf(template, test.specObject)),
							)
							if err == nil {
								break
							}
							time.Sleep(5 * time.Millisecond)
						}
						if err != nil {
							t.Error(err)
						}
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected http status 200, got %d", resp.StatusCode)
						}
						defer resp.Body.Close()

						var actualResp v1beta1.AdmissionReview
						err = json.NewDecoder(resp.Body).Decode(&actualResp)
						if err != nil {
							t.Errorf("unable to decode resp body: %s", err)
						}

						expectedInvalidResponse := v1beta1.AdmissionReview{
							Response: &v1beta1.AdmissionResponse{
								Result: &metav1.Status{
									Message: test.errorResponse,
								},
							},
						}
						if diff := cmp.Diff(expectedInvalidResponse, actualResp); diff != "" {
							t.Errorf("As (-want, +got) = %v", diff)
						}
					})
				}
			}
		})
		t.Run("Does not allow changing sink type", func(t *testing.T) {
			server := webhook.NewServer("127.0.0.1:0")
			server.Run(false)
			defer server.Close()

			for ttype, template := range map[string]string{
				"cluster":   clusterLogSinkUpdateAdmissionTemplate,
				"namespace": logSinkUpdateAdmissionTemplate,
			} {
				t.Run(ttype, func(t *testing.T) {
					var (
						err  error
						resp *http.Response
					)
					for i := 0; i < 100; i++ {
						resp, err = http.Post(
							"http://"+server.Addr()+"/logsink",
							"application/json",
							strings.NewReader(fmt.Sprintf(template,
								`{
									"type": "syslog",
									"host": "example.com",
									"port": 100
								}`,
								`{
									"type": "webhook",
									"url": "https://example.com/place"
								}`,
							)),
						)
						if err == nil {
							break
						}
						time.Sleep(5 * time.Millisecond)
					}
					if err != nil {
						t.Error(err)
					}
					if resp.StatusCode != http.StatusOK {
						t.Errorf("expected http status 200, got %d", resp.StatusCode)
					}
					defer resp.Body.Close()

					var actualResp v1beta1.AdmissionReview
					err = json.NewDecoder(resp.Body).Decode(&actualResp)
					if err != nil {
						t.Errorf("unable to decode resp body: %s", err)
					}

					expectedInvalidResponse := v1beta1.AdmissionReview{
						Response: &v1beta1.AdmissionResponse{
							Result: &metav1.Status{
								Message: "Changing sink type invalid",
							},
						},
					}
					if diff := cmp.Diff(expectedInvalidResponse, actualResp); diff != "" {
						t.Errorf("As (-want, +got) = %v", diff)
					}
				})
			}
		})
	})

	for ttype, template := range map[string]string{
		"Cluster":   clusterMetricAdmissionTemplate,
		"Namespace": metricAdmissionTemplate,
	} {
		t.Run(ttype+"_Metric_Sink", func(t *testing.T) {
			t.Run("returns an allowed admission response", func(t *testing.T) {
				requireTelegraf(t)
				server := webhook.NewServer("127.0.0.1:0")
				server.Run(false)
				defer server.Close()

				var (
					err  error
					resp *http.Response
				)
				for i := 0; i < 100; i++ {
					resp, err = http.Post(
						"http://"+server.Addr()+"/metricsink",
						"application/json",
						strings.NewReader(fmt.Sprintf(template,
							`{
							"inputs": [ {
								"commands": [ "echo", "5" ],
								"data_format": "value",
								"data_type": "integer",
								"name_override": "test",
								"type": "exec"
							} ],
							"outputs": [ {
								"apikey": "apikey",
								"type": "datadog"
							} ]
						}`)),
					)
					if err == nil {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err != nil {
					t.Error(err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected http status 200, got %d", resp.StatusCode)
				}
				defer resp.Body.Close()

				var actualResp v1beta1.AdmissionReview
				err = json.NewDecoder(resp.Body).Decode(&actualResp)
				if err != nil {
					t.Errorf("unable to decode resp body: %s", err)
				}

				if !actualResp.Response.Allowed {
					t.Errorf("expected response to be allowed, got false")
				}
			})

			t.Run("returns a disallowed admission response for", func(t *testing.T) {
				requireTelegraf(t)
				tests := []invalidValidationTest{
					{
						"user specified kubernetes input",
						`{
						"inputs": [ {
							"type": "kubernetes"
						} ],
						"outputs": [ {
							"apikey": "apikey",
							"type": "datadog"
						} ]
					}`,
						webhook.ConfigIncludesKubernetesError,
					},
					{
						"no input type",
						`{
						"inputs": [ {
						    "apikey": "apikey"
						} ]
					}`,
						webhook.ConfigMetricNoTypeError,
					},
					{
						"bad input type",
						`{
						"inputs": [ {
							"type": 123
						} ]
					}`,
						webhook.ConfigMetricNonStringTypeError,
					},
					{
						"no output type",
						`{
						"outputs": [ {
						    "apikey": "apikey"
						} ]
					}`,
						webhook.ConfigMetricNoTypeError,
					},
					{
						"bad output type",
						`{
						"outputs": [ {
							"type": 123
						} ]
					}`,
						webhook.ConfigMetricNonStringTypeError,
					},
					{
						"invalid output",
						`{
						"inputs": [ {
							"type": "cpu"
						} ],
						"outputs": [ {
							"type": "datadog",
							"garbage": "datadog"
						} ]
					}`,
						webhook.ConfigTelegrafError,
					},
				}
				if ttype == "Namespace" {
					tests = append(tests, invalidValidationTest{
						"no input",
						`{
						"outputs": [{
							"type": "datadog",
							"apikey": "apikey"
						}]
					}`,
						webhook.ConfigMetricNoInputError,
					})
				}
				server := webhook.NewServer("127.0.0.1:0")
				server.Run(false)
				defer server.Close()

				for _, test := range tests {
					t.Run(test.name, func(t *testing.T) {
						var (
							err  error
							resp *http.Response
						)
						for i := 0; i < 100; i++ {
							resp, err = http.Post(
								"http://"+server.Addr()+"/metricsink",
								"application/json",
								strings.NewReader(fmt.Sprintf(template, test.specObject)),
							)
							if err == nil {
								break
							}
							time.Sleep(5 * time.Millisecond)
						}
						if err != nil {
							t.Error(err)
						}
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected http status 200, got %d", resp.StatusCode)
						}
						defer resp.Body.Close()

						var actualResp v1beta1.AdmissionReview
						err = json.NewDecoder(resp.Body).Decode(&actualResp)
						if err != nil {
							t.Errorf("unable to decode resp body: %s", err)
						}

						expectedInvalidResponse := v1beta1.AdmissionReview{
							Response: &v1beta1.AdmissionResponse{
								Result: &metav1.Status{
									Message: test.errorResponse,
								},
							},
						}
						if diff := cmp.Diff(expectedInvalidResponse, actualResp); diff != "" {
							t.Errorf("As (-want, +got) = %v", diff)
						}
					})
				}
			})
		})
	}
}

var (
	admissionTemplate = `{
		"kind": "AdmissionReview",
		"apiVersion": "admission.k8s.io/v1beta1",
		"request": {
			"uid": "f9bc53a0-266b-11e9-928e-42010a800feb",
			"kind": {
				"group": "apps.pivotal.io",
				"version": "v1beta1",
				"kind": "%s"
			},
			"resource": {
				"group": "apps.pivotal.io",
				"version": "v1beta1",
				"resource": "%s"
			},
			"operation": "CREATE",
			"object": {
				"apiVersion": "apps.pivotal.io/v1beta1",
				"kind": "%[1]s",
				"spec": %%s
			}
		}
	}`
	updateAdmissionTemplate = `{
		"kind": "AdmissionReview",
		"apiVersion": "admission.k8s.io/v1beta1",
		"request": {
			"uid": "f9bc53a0-266b-11e9-928e-42010a800feb",
			"kind": {
				"group": "apps.pivotal.io",
				"version": "v1beta1",
				"kind": "%s"
			},
			"resource": {
				"group": "apps.pivotal.io",
				"version": "v1beta1",
				"resource": "%s"
			},
			"operation": "UPDATE",
			"object": {
				"apiVersion": "apps.pivotal.io/v1beta1",
				"kind": "%[1]s",
				"spec": %%s
			},
			"oldObject": {
				"apiVersion": "apps.pivotal.io/v1beta1",
				"kind": "%[1]s",
				"spec": %%s
			}
		}
	}`

	clusterLogSinkAdmissionTemplate = fmt.Sprintf(admissionTemplate, "ClusterLogSink", "clusterlogsinks")
	logSinkAdmissionTemplate        = fmt.Sprintf(admissionTemplate, "LogSink", "logsinks")
	clusterMetricAdmissionTemplate  = fmt.Sprintf(admissionTemplate, "ClusterMetricSink", "clustermetricsinks")
	metricAdmissionTemplate         = fmt.Sprintf(admissionTemplate, "MetricSink", "metricsinks")

	logSinkUpdateAdmissionTemplate        = fmt.Sprintf(updateAdmissionTemplate, "LogSink", "logsinks")
	clusterLogSinkUpdateAdmissionTemplate = fmt.Sprintf(updateAdmissionTemplate, "ClusterLogSink", "clusterlogsinks")
)
