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
package state_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	sink "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/state"
)

func TestClient(t *testing.T) {
	t.Run("returns states on successful request", func(t *testing.T) {
		server := httptest.NewServer(handler(successResponse, http.StatusOK))
		defer func() {
			server.CloseClientConnections()
			server.Close()
		}()

		c := state.NewClient(server.URL)

		actualStates, err := c.States()
		if err != nil {
			t.Errorf("Error getting states: %s", err)
		}

		if len(actualStates) != 1 {
			t.Errorf("Expected 1 state, got %d", len(actualStates))
		}

		if diff := cmp.Diff(expectedStates, actualStates); diff != "" {
			t.Errorf("As (-want, +got) = %v", diff)
		}
	})

	t.Run("returns error on non 200 status code", func(t *testing.T) {
		server := httptest.NewServer(handler("", http.StatusNotFound))
		defer func() {
			server.CloseClientConnections()
			server.Close()
		}()

		c := state.NewClient(server.URL)

		actualStates, err := c.States()
		if err == nil {
			t.Error("Expected to get error, got nil")
		}

		if actualStates != nil {
			t.Errorf("Expected states to be nil, got %+v", actualStates)
		}
	})

	t.Run("returns error if it fails to GET", func(t *testing.T) {
		c := state.NewClient("some-url")
		actualStates, err := c.States()
		if err == nil {
			t.Error("Expected to get error, got nil")
		}

		if actualStates != nil {
			t.Errorf("Expected states to be nil, got %+v", actualStates)
		}
	})

	t.Run("returns error if response body is invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(handler("[{", http.StatusOK))
		defer func() {
			server.CloseClientConnections()
			server.Close()
		}()

		c := state.NewClient(server.URL)

		actualStates, err := c.States()
		if err == nil {
			t.Error("Expected to get error, got nil")
		}

		if actualStates != nil {
			t.Errorf("Expected states to be nil, got %+v", actualStates)
		}
	})
}

func TestSinkState(t *testing.T) {
	t.Run("returns patch with Running state if sink state has no error", func(t *testing.T) {
		s := state.SinkState{
			Name:               "test-sink",
			Namespace:          "test-ns",
			LastSuccessfulSend: testTS,
		}

		b, err := s.PatchBytes()
		if err != nil {
			t.Errorf("Expected error to be nil, Got %s", err)
		}
		var actualPatch []patch
		err = json.Unmarshal(b, &actualPatch)
		if err != nil {
			t.Errorf("Expected error to be nil, Got %s", err)
		}

		if actualPatch[0].Value.State != sink.SinkStateRunning {
			t.Errorf("Expected state to be running, was %s", actualPatch[0].Value.State)
		}
	})

	t.Run("returns patch with Failing state if sink state has error", func(t *testing.T) {
		s := state.SinkState{
			Name:               "test-sink",
			Namespace:          "test-ns",
			LastSuccessfulSend: testTS,
			Error: &state.SinkError{
				Msg: "some-error",
			},
		}

		b, err := s.PatchBytes()
		if err != nil {
			t.Errorf("Expected error to be nil, Got %s", err)
		}
		var actualPatch []patch
		err = json.Unmarshal(b, &actualPatch)
		if err != nil {
			t.Errorf("Expected error to be nil, Got %s", err)
		}

		if actualPatch[0].Value.State != sink.SinkStateFailing {
			t.Errorf("Expected state to be running, was %s", actualPatch[0].Value.State)
		}
	})
}

type patch struct {
	Op    string
	Path  string
	Value sink.SinkStatus
}

var (
	successResponse = `[{
		"name": "test-sink",
		"namespace": "test-ns",
		"last_successful_send": "2009-11-10T23:00:00Z",
		"error": {
			"msg": "some-error"
		}
	}]`

	handler = func(body string, code int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, err := w.Write([]byte(body))
			if err != nil {
				panic(err)
			}
		}
	}

	testTS = time.Unix(1257894000, 0)

	expectedStates = []state.SinkState{
		{
			Name:               "test-sink",
			Namespace:          "test-ns",
			LastSuccessfulSend: testTS,
			Error: &state.SinkError{
				Msg: "some-error",
			},
		},
	}
)
