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
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LogSink is a specification for a LogSink resource
type LogSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SinkSpec `json:"spec"`
}

// SinkSpec is the spec for a Sink resource
type SinkSpec struct {
	Type string `json:"type"`

	SyslogSpec         `json:",inline"`
	WebhookSpec        `json:",inline"`
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

type SyslogSpec struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	EnableTLS bool   `json:"enable_tls"`
}

type WebhookSpec struct {
	URL string `json:"url"`
}

// SinkStatus is the status for a Sink resource
type SinkStatus struct {
	State              SinkState         `json:"state,omitempty"`
	LastSuccessfulSend metav1.MicroTime  `json:"last_successful_send,omitempty"`
	LastError          *string           `json:"last_error,omitempty"`
	LastErrorTime      *metav1.MicroTime `json:"last_error_time,omitempty"`
}

type SinkState string

const (
	SinkStateRunning SinkState = "Running"
	SinkStateFailing SinkState = "Failing"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LogSinkList is a list of LogSink resources
type LogSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LogSink `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterLogSink is a specification for a ClusterLogSink resource
type ClusterLogSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SinkSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterLogSinkList is a list of ClusterLogSink resources
type ClusterLogSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterLogSink `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterMetricSink is a specification for a ClusterMetricSink resource
type ClusterMetricSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   MetricSinkSpec `json:"spec"`
	Status SinkStatus     `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MetricSink is a specification for a MetricSink resource
type MetricSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   MetricSinkSpec `json:"spec"`
	Status SinkStatus     `json:"status,omitempty"`
}

// MetricSinkSpec is the spec for a Sink resource
type MetricSinkSpec struct {
	Inputs  []MetricSinkMap `json:"inputs"`
	Outputs []MetricSinkMap `json:"outputs"`
}

// MetricSinkMap contains key/values that define inputs and outputs for a
// MetricSink.
type MetricSinkMap map[string]interface{}

func (m MetricSinkMap) DeepCopy() MetricSinkMap {
	newMap := make(MetricSinkMap, len(m))
	for k, v := range m {
		switch tv := v.(type) {
		case string:
			newMap[k] = tv
		case int:
			newMap[k] = tv
		}
	}
	return newMap
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterMetricSinkList is a list of ClusterMetricSink resources
type ClusterMetricSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterMetricSink `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MetricSinkList is a list of MetricSink resources
type MetricSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MetricSink `json:"items"`
}
