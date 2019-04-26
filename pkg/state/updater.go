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
	"log"
	"time"

	sink "github.com/knative/observability/pkg/apis/sink/v1alpha1"
	"github.com/knative/observability/pkg/client/clientset/versioned/typed/sink/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type ClientGetter interface {
	v1alpha1.LogSinksGetter
	v1alpha1.ClusterLogSinksGetter
}

type SinksStateUpdater struct {
	clientGetter ClientGetter
}

func NewUpdater(cg ClientGetter) *SinksStateUpdater {
	return &SinksStateUpdater{
		clientGetter: cg,
	}
}

func (u *SinksStateUpdater) Update(states []SinkState) {
	for _, s := range states {
		u.patchStatus(s)
	}
}

func (u *SinksStateUpdater) patchStatus(s SinkState) {
	if s.Namespace == "" {
		err := u.patchClusterLogSink(s)
		if err != nil {
			log.Printf(
				"Unable to patch status for ClusterLogSink (%s): %s",
				s.Name,
				err,
			)
		}
		return
	}

	err := u.patchLogSink(s)
	if err != nil {
		log.Printf(
			"Unable to patch status for LogSink (%s): %s",
			s.Name,
			err,
		)
	}
}

func (u *SinksStateUpdater) patchLogSink(s SinkState) error {
	b, err := s.PatchBytes()
	if err != nil {
		return err
	}

	_, err = u.clientGetter.LogSinks(s.Namespace).Patch(
		s.Name,
		types.JSONPatchType,
		b,
		"status",
	)

	return err
}

func (u *SinksStateUpdater) patchClusterLogSink(s SinkState) error {
	b, err := s.PatchBytes()
	if err != nil {
		return err
	}

	_, err = u.clientGetter.ClusterLogSinks(s.Namespace).Patch(
		s.Name,
		types.JSONPatchType,
		b,
		"status",
	)

	return err
}

func FilterStaleStates(olderThan time.Duration, states []SinkState) []SinkState {
	newTime := time.Now().Add(-olderThan)
	stateChanged := make([]SinkState, 0)
	for _, s := range states {
		if s.LastSuccessfulSend.After(newTime) || (s.Error != nil && s.Error.Timestamp.After(newTime)) {
			stateChanged = append(stateChanged, s)
		}
	}
	return stateChanged
}

type patch struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value sink.SinkStatus `json:"value"`
}
