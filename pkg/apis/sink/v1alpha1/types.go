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

	Spec   SinkSpec   `json:"spec"`
	Status SinkStatus `json:"status,omitempty"`
}

// SinkSpec is the spec for a Sink resource
type SinkSpec struct {
	Type               string `json:"type"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	EnableTLS          bool   `json:"enable_tls"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

// SinkStatus is the status for a Sink resource
type SinkStatus struct {
	State   SinkState `json:"state,omitempty"`
	Message string    `json:"message,omitempty"`
}

type SinkState string

const (
	SinkStateCreated   SinkState = "Created"
	SinkStateProcessed SinkState = "Processed"
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

	Spec   SinkSpec   `json:"spec"`
	Status SinkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterLogSinkList is a list of ClusterLogSink resources
type ClusterLogSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterLogSink `json:"items"`
}
