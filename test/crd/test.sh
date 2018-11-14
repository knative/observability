#!/bin/bash

# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eeo pipefail; [ -n "$DEBUG" ] && set -x; set -u

working_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

dupes="$(
    grep -ir "name:" "$working_dir"/*/*.yaml | \
    awk '{print $NF}' | \
    sort | \
    uniq -c | \
    sort -r
)"
if [ "$(echo "$dupes" | awk '{print $1}' | sort -u | wc -l)" -ne 1 ]; then
    echo "All test objects do not have unique names:"
    echo "$dupes"
    exit 1
fi

function cleanup {
    for f in "$working_dir/valid"/*; do
        kubectl delete -f "$f" > /dev/null 2>&1
    done
    if [ "$created_log_sink_crd" -eq 0 ]; then
        kubectl delete -f "$working_dir/../config/100-log-sink-crd.yaml" > /dev/null 2>&1
    fi
    if [ "$created_cluster_log_sink_crd" -eq 0 ]; then
        kubectl delete -f "$working_dir/../config/100-cluster-log-sink-crd.yaml" > /dev/null 2>&1
    fi
}
trap cleanup EXIT


set +e
kubectl create -f "$working_dir/../config/100-log-sink-crd.yaml" > /dev/null 2>&1
created_log_sink_crd=$?
kubectl create -f "$working_dir/../config/100-cluster-log-sink-crd.yaml" > /dev/null 2>&1
created_cluster_log_sink_crd=$?
set -e

failed=false

for f in "$working_dir/valid"/*; do
    if out="$(kubectl apply -f "$f" 2>&1)"; then
        echo "PASSED: valid/$(basename "$f")"
    else
        echo "FAILED: valid/$(basename "$f")"
        echo "$out"
        failed=true
    fi
done

for f in "$working_dir/invalid"/*; do
    if out="$(kubectl apply -f "$f" 2>&1)"; then
        echo "FAILED: invalid/$(basename "$f")"
        echo "$out"
        failed=true
    else
        echo "PASSED: invalid/$(basename "$f")"
    fi
done

if [ "$failed" != false ]; then
    exit 1
fi
