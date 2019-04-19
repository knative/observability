package sink_test

import (
	"testing"

	"github.com/knative/observability/pkg/sink"
)

func TestSetClusterNameFilter(t *testing.T) {
	spyConfigMapPatcher := &spyConfigMapPatcher{}
	spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}

	sink.SetClusterNameFilter(
		spyConfigMapPatcher,
		spyDaemonSetPodDeleter,
		"test-cluster-name",
	)

	expectedPatch := []spyPatch{
		{
			Path:  "/data/cluster-name-filter.conf",
			Value: "\n[FILTER]\n    Name record_modifier\n    Match *\n    Record cluster_name test-cluster-name\n",
		},
	}

	spyConfigMapPatcher.expectPatches(expectedPatch, t)
	if spyDaemonSetPodDeleter.Selector != "app=fluent-bit" {
		t.Errorf("DaemonSet PodDeleter not equal: Expected: %s, Actual: %s", spyDaemonSetPodDeleter.Selector, "app=fluent-bit")
	}
}

func TestSetClusterNameFilterIgnoresEmptyClustername(t *testing.T) {
	spyConfigMapPatcher := &spyConfigMapPatcher{}
	spyDaemonSetPodDeleter := &spyDaemonSetPodDeleter{}

	sink.SetClusterNameFilter(
		spyConfigMapPatcher,
		spyDaemonSetPodDeleter,
		"",
	)

	if spyConfigMapPatcher.patchCalled {
		t.Error("Patch should not be called for empty cluster name")
	}

	if spyDaemonSetPodDeleter.deleteCollectionCalled {
		t.Error("Delete collection should not be called for empty cluster name")
	}
}
