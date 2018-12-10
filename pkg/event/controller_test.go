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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"github.com/knative/observability/pkg/event"
)

var _ = Describe("Event", func() {

	BeforeEach(func() {
		event.ForwarderSent.Set(0)
		event.ForwarderFailed.Set(0)
		event.ForwarderConvertFailed.Set(0)
	})

	It("forwards event to fluent bit when an event is added", func() {
		spyFl := &spyFlogger{}
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

		Expect(spyFl.receivedMsg).To(Equal(expected))
		Expect(spyFl.tag).To(Equal("k8s.event"))
		Expect(event.ForwarderSent.Value()).To(BeEquivalentTo(1))
	})

	It("does nothing when OnUpdate is called", func() {
		spyFl := &spyFlogger{}
		c := event.NewController(spyFl)
		c.OnUpdate(nil, nil)
		Expect(spyFl.called).To(BeFalse())
	})

	It("does nothing when OnDelete is called", func() {
		spyFl := &spyFlogger{}
		c := event.NewController(spyFl)
		c.OnDelete(nil)
		Expect(spyFl.called).To(BeFalse())
	})

	It("does not forward when non v1 event is received", func() {
		spyFl := &spyFlogger{}
		c := event.NewController(spyFl)

		c.OnAdd("non-v1-event")
		Expect(spyFl.called).To(BeFalse())
		Expect(event.ForwarderSent.Value()).To(BeEquivalentTo(0))
		Expect(event.ForwarderConvertFailed.Value()).To(BeEquivalentTo(1))
	})

	It("does not forward when forwarder fails to post", func() {
		spyFl := &spyFlogger{
			err: errors.New("some error"),
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

		Expect(event.ForwarderSent.Value()).To(BeEquivalentTo(0))
		Expect(event.ForwarderFailed.Value()).To(BeEquivalentTo(1))
	})

	It("handles empty Source", func() {
		spyFl := &spyFlogger{}
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

		Expect(spyFl.receivedMsg).To(Equal(expected))
		Expect(spyFl.tag).To(Equal("k8s.event"))
	})
})

type spyFlogger struct {
	err         error
	called      bool
	tag         string
	receivedMsg map[string]interface{}
}

func (s *spyFlogger) Post(tag string, message interface{}) error {
	s.called = true
	if s.err != nil {
		return s.err
	}
	s.tag = tag
	msg, ok := message.(map[string]interface{})
	Expect(ok).To(BeTrue())

	s.receivedMsg = msg

	return nil
}
