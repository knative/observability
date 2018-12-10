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

set -Eeuo pipefail

# This script is intended to be ran with go modules. update-codegen.sh was
# intended to run with a go workspace and have all sources under the GOPATH.

SCRIPT_ROOT="$(dirname "$BASH_SOURCE")/.."

# setup GOPATH to a temp dir as it is needed for update-codegen.sh
export GOPATH="$(mktemp -d)"

# copy over the vendor dir that contains k8s.io/code-generator
rsync -a "$SCRIPT_ROOT/vendor/" "$GOPATH/src/"

export CODEGEN_PKG="$SCRIPT_ROOT/vendor/k8s.io/code-generator"
"$SCRIPT_ROOT/hack/update-codegen.sh"

# copy over the generated code
rsync -a "$GOPATH/src/github.com/knative/observability/pkg/" "$SCRIPT_ROOT/pkg/"
