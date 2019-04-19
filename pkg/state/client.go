/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package state

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	sink "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SinkError struct {
	Msg       string    `json:"msg"`
	Timestamp time.Time `json:"timestamp"`
}

type SinkState struct {
	Name               string     `json:"name"`
	Namespace          string     `json:"namespace"`
	LastSuccessfulSend time.Time  `json:"last_successful_send"`
	Error              *SinkError `json:"error"`
}

func (s SinkState) PatchBytes() ([]byte, error) {
	state := sink.SinkStateRunning
	var errorMsg *string
	var errorTime *metav1.MicroTime
	if s.Error != nil {
		state = sink.SinkStateFailing
		errorMsg = &s.Error.Msg
		tmp := metav1.NewMicroTime(s.Error.Timestamp)
		errorTime = &tmp
	}
	return json.Marshal([]patch{
		{
			Op:   "add",
			Path: "/status",
			Value: sink.SinkStatus{
				State:              state,
				LastSuccessfulSend: metav1.NewMicroTime(s.LastSuccessfulSend),
				LastError:          errorMsg,
				LastErrorTime:      errorTime,
			},
		},
	})
}

type ClientOpt func(*Client)

// WithHTTPClient can be used to configure the http client.
func WithHTTPClient(h *http.Client) ClientOpt {
	return func(c *Client) {
		c.httpClient = h
	}
}

// Client handles getting states from the syslog state endpoint.
type Client struct {
	url        string
	httpClient *http.Client
}

// NewClient returns a Client to request States. It uses http.DefaultClient
// which can be overriden with ClientOpts.
func NewClient(url string, opts ...ClientOpt) *Client {
	c := &Client{
		url:        url,
		httpClient: http.DefaultClient,
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

// States gets a list of SinkState from the configured url.
func (c *Client) States() ([]SinkState, error) {
	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to get states, expected status 200, got %d", resp.StatusCode)
	}

	var states []SinkState
	err = json.NewDecoder(resp.Body).Decode(&states)
	if err != nil {
		return nil, err
	}

	return states, nil
}
