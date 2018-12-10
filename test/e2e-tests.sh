#!/usr/bin/env bash

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

# This script runs the end-to-end tests for observability components built
# from source. It is started by prow for each PR. For convenience, it can also
# be executed manually.

# If you already have the *_OVERRIDE environment variables set, call this
# script with the --run-tests arguments and it will use the cluster and run
# the tests.

# Calling this script without arguments will create a new cluster in project
# $PROJECT_ID, start the controller, run the tests and delete the cluster.

source "$(dirname "$0")/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh"

function teardown() {
  ko delete --ignore-not-found=true -R -f test/ || true
  ko delete --ignore-not-found=true -f config/ || true
}

initialize $@

# Fail fast during setup.
set -o errexit
set -o pipefail

header "Building and starting observability components"
export KO_DOCKER_REPO="$DOCKER_REPO_OVERRIDE"
ko apply -f config/ || fail_test

# Handle test failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

# Make sure that are no builds or build templates in the current namespace.
wait_until_pods_running knative-observability || fail_test

# Run the tests
header "Running CRD e2e tests"
"$(dirname "$0")/crd/test.sh" || fail_test

header "Running Go e2e tests"
go_test_e2e ./test/e2e/... || fail_test

success
