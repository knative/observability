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
	"errors"
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/client/clientset/versioned/fake"
	"github.com/knative/observability/pkg/state"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8stesting "k8s.io/client-go/testing"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func TestUpdate(t *testing.T) {
	t.Run("updates the logsink and cluster logsink status", func(t *testing.T) {
		client := fake.NewSimpleClientset(&v1alpha1.LogSink{}, &v1alpha1.ClusterLogSink{})
		u := state.NewUpdater(client.ObservabilityV1alpha1())
		u.Update(sinkStates)

		if len(client.Actions()) != 2 {
			t.Errorf("Expected 2 action got %d", len(client.Actions()))
		}

		expected := []k8stesting.Action{
			k8stesting.PatchActionImpl{
				ActionImpl: k8stesting.ActionImpl{
					Namespace: sinkStates[0].Namespace,
					Verb:      "patch",
					Resource: schema.GroupVersionResource{
						Group:    "observability.knative.dev",
						Version:  "v1alpha1",
						Resource: "logsinks",
					},
					Subresource: "status",
				},
				Name:      sinkStates[0].Name,
				PatchType: types.JSONPatchType,
				Patch:     []byte(`[{"op":"add","path":"/status","value":{"state":"Running","last_successful_send":"2009-11-10T23:00:00.000000Z"}}]`),
			},
			k8stesting.PatchActionImpl{
				ActionImpl: k8stesting.ActionImpl{
					Namespace: sinkStates[1].Namespace,
					Verb:      "patch",
					Resource: schema.GroupVersionResource{
						Group:    "observability.knative.dev",
						Version:  "v1alpha1",
						Resource: "clusterlogsinks",
					},
					Subresource: "status",
				},
				Name:      sinkStates[1].Name,
				PatchType: types.JSONPatchType,
				Patch:     []byte(`[{"op":"add","path":"/status","value":{"state":"Running","last_successful_send":"2009-11-10T23:00:00.000000Z"}}]`),
			},
		}

		if diff := cmp.Diff(expected, client.Actions(), byteTransformer); diff != "" {
			t.Errorf("As (-want, +got)\n%v", diff)
		}
	})

	t.Run("has a failing state when there is an error", func(t *testing.T) {
		client := fake.NewSimpleClientset(&v1alpha1.LogSink{}, &v1alpha1.ClusterLogSink{})
		u := state.NewUpdater(client.ObservabilityV1alpha1())
		u.Update(failingSinkState)

		if len(client.Actions()) != 2 {
			t.Errorf("Expected 2 action got %d", len(client.Actions()))
		}

		expected := []k8stesting.Action{
			k8stesting.PatchActionImpl{
				ActionImpl: k8stesting.ActionImpl{
					Namespace: sinkStates[0].Namespace,
					Verb:      "patch",
					Resource: schema.GroupVersionResource{
						Group:    "observability.knative.dev",
						Version:  "v1alpha1",
						Resource: "logsinks",
					},
					Subresource: "status",
				},
				Name:      sinkStates[0].Name,
				PatchType: types.JSONPatchType,
				Patch:     []byte(`[{"op":"add","path":"/status","value":{"state":"Failing","last_successful_send":"2009-11-10T23:00:00.000000Z","last_error_time":"2009-11-10T23:00:00.000000Z","last_error":"err1"}}]`),
			},
			k8stesting.PatchActionImpl{
				ActionImpl: k8stesting.ActionImpl{
					Namespace: sinkStates[1].Namespace,
					Verb:      "patch",
					Resource: schema.GroupVersionResource{
						Group:    "observability.knative.dev",
						Version:  "v1alpha1",
						Resource: "clusterlogsinks",
					},
					Subresource: "status",
				},
				Name:      sinkStates[1].Name,
				PatchType: types.JSONPatchType,
				Patch:     []byte(`[{"op":"add","path":"/status","value":{"state":"Failing","last_successful_send":"2009-11-10T23:00:00.000000Z","last_error_time":"2009-11-10T23:00:01.000000Z","last_error":"err2"}}]`),
			},
		}

		if diff := cmp.Diff(expected, client.Actions(), byteTransformer); diff != "" {
			t.Errorf("As (-want, +got)\n%v", diff)
		}
	})

	t.Run("continues to patch next sink state if one fails to patch", func(t *testing.T) {
		client := fake.NewSimpleClientset(&v1alpha1.LogSink{}, &v1alpha1.ClusterLogSink{})
		client.PrependReactor("patch", "logsinks", k8stesting.ReactionFunc(errorReactionFunc))
		client.PrependReactor("patch", "clusterlogsinks", k8stesting.ReactionFunc(errorReactionFunc))

		u := state.NewUpdater(client.ObservabilityV1alpha1())
		u.Update([]state.SinkState{
			{
				Name:               "test-sink",
				Namespace:          "test-ns",
				LastSuccessfulSend: testTS,
			},
			{
				Name:               "cluster-test-sink",
				Namespace:          "",
				LastSuccessfulSend: testTS,
			},
		})

		if len(client.Actions()) != 2 {
			t.Errorf("Expected 2 action got %d", len(client.Actions()))
		}
		client.ClearActions()

		u.Update([]state.SinkState{
			{
				Name:               "cluster-test-sink",
				Namespace:          "",
				LastSuccessfulSend: testTS,
			},
			{
				Name:               "test-sink",
				Namespace:          "test-ns",
				LastSuccessfulSend: testTS,
			},
		})

		if len(client.Actions()) != 2 {
			t.Errorf("Expected 2 action got %d", len(client.Actions()))
		}
	})
}

func TestFilterStaleStates(t *testing.T) {
	now := time.Now()
	sinkStates = []state.SinkState{
		{
			Name:               "new-success",
			LastSuccessfulSend: now,
		},
		{
			Name:               "old success",
			LastSuccessfulSend: now.Add(-time.Hour),
		},
		{
			Name:               "new-failure",
			LastSuccessfulSend: now.Add(-time.Hour),
			Error: &state.SinkError{
				Msg:       "err1",
				Timestamp: now,
			},
		},
		{
			Name:               "old-failure",
			LastSuccessfulSend: now.Add(-time.Hour),
			Error: &state.SinkError{
				Msg:       "err1",
				Timestamp: now.Add(-time.Hour),
			},
		},
	}

	expected := []state.SinkState{
		{
			Name:               "new-success",
			LastSuccessfulSend: now,
		},
		{
			Name:               "new-failure",
			LastSuccessfulSend: now.Add(-time.Hour),
			Error: &state.SinkError{
				Msg:       "err1",
				Timestamp: now,
			},
		},
	}

	filteredStates := state.FilterStaleStates(time.Minute, sinkStates)
	if diff := cmp.Diff(expected, filteredStates); diff != "" {
		t.Errorf("As (-want, +got)\n%v", diff)
	}
}

var (
	sinkStates = []state.SinkState{
		{
			Name:               "test-sink",
			Namespace:          "test-ns",
			LastSuccessfulSend: testTS,
		},
		{
			Name:               "cluster-test-sink",
			Namespace:          "",
			LastSuccessfulSend: testTS,
		},
	}

	failingSinkState = []state.SinkState{
		{
			Name:               "test-sink",
			Namespace:          "test-ns",
			LastSuccessfulSend: testTS,
			Error: &state.SinkError{
				Msg:       "err1",
				Timestamp: time.Unix(1257894000, 0), // 2009-11-10T23:00:00.000000Z
			},
		},
		{
			Name:               "cluster-test-sink",
			Namespace:          "",
			LastSuccessfulSend: testTS,
			Error: &state.SinkError{
				Msg:       "err2",
				Timestamp: time.Unix(1257894001, 0), // 2009-11-10T23:00:01.000000Z
			},
		},
	}

	errorReactionFunc = func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("an-error")
	}
)

var byteTransformer = cmp.Transformer("jsonByteStringer", func(x []byte) interface{} {
	var tmp interface{}
	err := json.Unmarshal(x, &tmp)
	if err != nil {
		return string(x)
	}
	return tmp
})
