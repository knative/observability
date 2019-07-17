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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/release.sh

# Local generated yaml file
readonly OUTPUT_YAML=build.yaml

function build_release() {
  local image_base_path="${KO_DOCKER_REPO}/github.com/knative/observability/cmd"
  local image_tag=""

  if [[ -n "${TAG:-}" ]]; then
      image_tag="$TAG"
  else
      image_tag="$(git rev-parse HEAD)"
  fi

  local manifest_substitutions=""

  for image_name in cert-generator validator; do
    echo "Building $image_name"

    local image_path="${image_base_path}/$image_name:$image_tag"

    docker build -t $image_path -f cmd/$image_name/Dockerfile .

    if [[ -z "${TAG:-}" ]]; then
      manifest_image_reference="$image_path"
    else
      manifest_image_reference="$(docker image inspect $image_path | jq -r '.[0].RepoDigests[0]')"
    fi

    manifest_substitutions="${manifest_substitutions};s|oratos/${image_name}:v.*|${manifest_image_reference}|"

    if (( PUBLISH_RELEASE )); then
      docker push $image_path
    fi
  done

  echo "Building build-crd"
  ko resolve ${KO_FLAGS} -f config/ \
      | sed -e "${manifest_substitutions}" \
      > ${OUTPUT_YAML}
  YAMLS_TO_PUBLISH="${OUTPUT_YAML}"
}

main $@
