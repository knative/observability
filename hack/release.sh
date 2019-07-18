#!/usr/bin/env bash

# Copyright 2019 The Knative Authors
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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/release.sh

# Local generated yaml file
readonly OUTPUT_YAML=observability.yaml

function build_release() {
  local image_base_path="${KO_DOCKER_REPO}/github.com/knative/observability/cmd"
  local image_tag="${TAG:-$BUILD_TAG}"

  local manifest_substitutions=""

  for dockerfile in cmd/*/Dockerfile; do
    image_name=$(echo $dockerfile | cut -d / -f 2)
    echo "Building $image_name"

    local image_path="${image_base_path}/$image_name:$image_tag"

    docker build -t $image_path -f cmd/$image_name/Dockerfile .

    if [[ -z "${TAG:-}" ]]; then
      manifest_image_reference="$image_path"
    else
      manifest_image_reference="$(docker image inspect $image_path | jq -r '.[0].RepoDigests[0]')"
    fi

    # to replace images that are not built by ko in manifest
    manifest_substitutions="${manifest_substitutions};/image: /s|oratos/${image_name}:v.*|${manifest_image_reference}|"

    if (( PUBLISH_RELEASE )); then
      docker push $image_path
    fi
  done

  echo "Building build-crd"
  ko resolve ${KO_FLAGS} -f config/ \
      | sed -e "${manifest_substitutions}" \
      > ${OUTPUT_YAML}
  ARTIFACTS_TO_PUBLISH="${OUTPUT_YAML}"
}

main $@
