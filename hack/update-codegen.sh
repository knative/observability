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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

export GO111MODULE=on

# XXX: code-generator doesn't place nice with go modoules yet. So we need to
# setup a GOPATH with the code-generator. After the code-generator gets a
# chance to run in the new/temp GOPATH, we'll have to move the result into the
# current directory.
temp_dir=$(mktemp -d)

# Use go.mod to calculate which version of k8s.io/code-generator we want to
# use.
code_generator_version=$(go mod graph | awk '{print $2}' | grep 'k8s.io/code-generator' | cut -d '@' -f2)

mkdir -p $temp_dir/src/k8s.io
git clone https://github.com/kubernetes/code-generator $temp_dir/src/k8s.io/code-generator

pushd $temp_dir/src/k8s.io/code-generator
    # Figure out what version we should check out.
    if [[ $code_generator_version == *"-"* ]]; then
        # non-semver
        sha=$(echo $code_generator_version | cut -d '-' -f3)
        git checkout $sha
    else
        # semver
        git checkout $code_generator_version
    fi

    # Install deps
    GOPATH=$temp_dir go get ./...
popd

mkdir -p $temp_dir/src/github.com/knative/observability
cp -r $SCRIPT_ROOT/ $temp_dir/src/github.com/knative/observability

pushd $temp_dir/src/github.com/knative/observability/
	export GOPATH=$temp_dir

	${GOPATH}/src/k8s.io/code-generator/generate-groups.sh "deepcopy,client,informer,lister" \
	  github.com/knative/observability/pkg/client github.com/knative/observability/pkg/apis \
	  sink:v1alpha1 \
	  --go-header-file ${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt

    # Looks like everything went well. Time for the scary part. Move the temp
    # directory onto the current one.
    rm -rf ${SCRIPT_ROOT}/pkg/client/
    cp -r pkg/client/ ${SCRIPT_ROOT}/pkg/client/
popd
