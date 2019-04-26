package sink

import "fmt"

const clusterNameFilterTemplate = `
[FILTER]
    Name record_modifier
    Match *
    Record cluster_name %s
`

func SetClusterNameFilter(
	cmp ConfigMapPatcher,
	dsp DaemonSetPodDeleter,
	clusterName string,
) {
	if clusterName == "" {
		return
	}

	patchConfig([]patch{
		{
			Op:    "replace",
			Path:  "/data/cluster-name-filter.conf",
			Value: fmt.Sprintf(clusterNameFilterTemplate, clusterName),
		},
	}, cmp, dsp)
}
