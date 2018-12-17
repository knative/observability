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
package event_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"k8s.io/api/core/v1"

	"github.com/knative/observability/pkg/event"
)

func TestForwarding(t *testing.T) {
	ResetForwarderMetrics()
	spyFl := &spyFlogger{
		t: t,
	}
	c := event.NewController(spyFl)
	ev := &v1.Event{
		InvolvedObject: v1.ObjectReference{
			Name:      "some object name",
			Namespace: "some namespace",
		},
		Message: "some note with log data",
		Source: v1.EventSource{
			Host: "some host",
		},
	}

	expected := map[string]interface{}{
		"log":    []byte("some note with log data"),
		"stream": []byte("stdout"),
		"kubernetes": map[string]interface{}{
			"host":           []byte("some host"),
			"pod_name":       []byte("some object name"),
			"namespace_name": []byte("some namespace"),
		},
	}

	c.OnAdd(ev)

	if diff := cmp.Diff(spyFl.receivedMsg, expected); diff != "" {
		t.Errorf("Unexpected messages (-want +got): %v", diff)
	}
	if spyFl.tag != "k8s.event" {
		t.Errorf("Expected tag to be k8s.event, was %s", spyFl.tag)
	}
	if event.ForwarderSent.Value() != 1 {
		t.Errorf("Expected events sent to be 1, was %d", event.ForwarderSent.Value())
	}
}

func TestNoopUpdate(t *testing.T) {
	spyFl := &spyFlogger{
		t: t,
	}
	c := event.NewController(spyFl)
	c.OnUpdate(nil, nil)
	if spyFl.called {
		t.Errorf("Expected not to call Flogger")
	}
}

func TestNoopDelete(t *testing.T) {
	spyFl := &spyFlogger{
		t: t,
	}
	c := event.NewController(spyFl)
	c.OnDelete(nil)
	if spyFl.called {
		t.Errorf("Expected not to call Flogger")
	}
}

func TestNonV1Event(t *testing.T) {
	ResetForwarderMetrics()
	spyFl := &spyFlogger{
		t: t,
	}
	c := event.NewController(spyFl)

	c.OnAdd("non-v1-event")
	if spyFl.called {
		t.Errorf("Expected not to call Flogger")
	}

	if event.ForwarderSent.Value() != 0 {
		t.Errorf("Expected to not send event, sent %d", event.ForwarderSent.Value())
	}

	if event.ForwarderConvertFailed.Value() != 1 {
		t.Errorf("Expected to fail to convert send event")
	}
}

func TestFailToPost(t *testing.T) {
	ResetForwarderMetrics()
	spyFl := &spyFlogger{
		err: errors.New("some error"),
		t:   t,
	}
	c := event.NewController(spyFl)
	ev := &v1.Event{
		InvolvedObject: v1.ObjectReference{
			Name:      "some object name",
			Namespace: "some namespace",
		},
		Message: "some note with log data",
		Source: v1.EventSource{
			Host: "some host",
		},
	}

	c.OnAdd(ev)

	if event.ForwarderSent.Value() != 0 {
		t.Errorf("Expected not to send event, sent %d", event.ForwarderSent.Value())
	}
	if event.ForwarderFailed.Value() != 1 {
		t.Errorf("Expected to fail to forward a event")
	}
}

func TestEmptySource(t *testing.T) {
	spyFl := &spyFlogger{
		t: t,
	}
	c := event.NewController(spyFl)
	ev := &v1.Event{
		InvolvedObject: v1.ObjectReference{
			Name:      "some object name",
			Namespace: "some namespace",
		},
		Message: "some note with log data",
	}

	expected := map[string]interface{}{
		"log":    []byte("some note with log data"),
		"stream": []byte("stdout"),
		"kubernetes": map[string]interface{}{
			"host":           []byte(""),
			"pod_name":       []byte("some object name"),
			"namespace_name": []byte("some namespace"),
		},
	}

	c.OnAdd(ev)

	if diff := cmp.Diff(spyFl.receivedMsg, expected); diff != "" {
		t.Errorf("Unexpected messages (-want +got): %v", diff)
	}
	if spyFl.tag != "k8s.event" {
		t.Errorf("Expected tag to be k8s.event, was %s", spyFl.tag)
	}
}

func ResetForwarderMetrics() {
	event.ForwarderSent.Set(0)
	event.ForwarderFailed.Set(0)
	event.ForwarderConvertFailed.Set(0)
}

type spyFlogger struct {
	err         error
	called      bool
	tag         string
	receivedMsg map[string]interface{}
	t           *testing.T
}

func (s *spyFlogger) Post(tag string, message interface{}) error {
	s.called = true
	if s.err != nil {
		return s.err
	}
	s.tag = tag
	msg, ok := message.(map[string]interface{})
	if !ok {
		s.t.Errorf("message not a map")
	}

	s.receivedMsg = msg

	return nil
}
