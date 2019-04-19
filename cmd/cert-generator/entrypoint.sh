#!/bin/bash

set -Eeuo pipefail

function validate_env {
    if [ -z "${NAMESPACE:-}" ]; then
        echo NAMESPACE must be set
        exit 1
    fi
    if [ -z "${CA_CERT_NAME:-}" ]; then
        echo CA_CERT_NAME must be set
        exit 1
    fi
    if [ -z "${CERTS_TO_GENERATE:-}" ]; then
        echo "CERTS_TO_GENERATE must be set."
        echo "It is a list of cert names to be generated."
        echo "It should delimited by semi-colon."
        exit 1
    fi
}

# detect_invalid_cert_state will talk to the kubernetes API and detect if the
# secret state is valid. The state is invalid if the CA secret does not exist
# and any of the certs do exist.
function detect_invalid_cert_state {
    if ! secret_exists "$CA_CERT_NAME"; then
        IFS=';' read -ra certs <<< "$CERTS_TO_GENERATE"
        for name in "${certs[@]}"; do
            if secret_exists "$name"; then
                echo "Cluster TLS secrets are in a bad state. Try deleting them and re-generating."
                exit 1
            fi
        done
    fi
}

# secret_exists talks to the kubernetes API and checks if the named secret
# exists.
function secret_exists {
    local name="${1?}"
    kubectl get secret "$name" \
        --namespace "$NAMESPACE" \
            > /dev/null 2>&1
}

# create_cert will create a cert/key pair signed by the CA for the named cert.
function create_cert {
    local name="${1?}"

    echo '
    {
      "signing": {
        "default": {
          "expiry": "26280h"
        },
        "profiles": {
          "observability": {
            "usages": ["server auth", "client auth"],
            "expiry": "26280h"
          }
        }
      }
    }
    ' > ca_config.json

    echo '
    {
      "CN": "'"$name"'",
      "hosts": ["'"$name"'.'"$NAMESPACE"'.svc", "'"$name"'.'"$NAMESPACE"'.svc.cluster.local"],
      "key": {
        "algo": "rsa",
        "size": 4096
      }
    }' \
        | cfssl gencert \
            -ca="$CA_CERT_NAME".crt \
            -ca-key="$CA_CERT_NAME".key \
            -config=ca_config.json \
            -profile=observability \
            - \
        | cfssljson -bare "$name"

    echo "Generated cert for $name"
    mv "$name".pem "$name".crt
    mv "$name"-key.pem "$name".key
    rm "$name".csr
}

# create_ca will create a cert/key pair for the CA.
function create_ca {
    local name="$CA_CERT_NAME"
    echo '
    {
      "CN": "'"$name"'",
      "hosts": ["'"$name"'.'"$NAMESPACE"'.svc"],
      "key": {
        "algo": "rsa",
        "size": 4096
      }
    }' \
        | cfssl gencert -initca - \
        | cfssljson -bare "$name"

    echo "Generated CA for $name"
    mv "$name".pem "$name".crt
    mv "$name"-key.pem "$name".key
    rm "$name".csr
}

# create secret will upload the named cert and key to the kubernetes API.
function create_secret {
    local name="${1?}"
    echo "Creating k8s secret for $name"
    kubectl create secret tls "$name" \
        --namespace "$NAMESPACE" \
        --cert "$name".crt \
        --key "$name".key
}

# read_secret will read the data secret from the kubernetes API
function read_secret {
    local name="${1?}"
    kubectl get secret "$name" \
        --namespace "$NAMESPACE" \
        --output json \
            | jq .data
}

# ensure_ca ensures that the cert and key for the CA exist both in the
# kubernetes API and locally on disk.
function ensure_ca {
    if ! secret_exists "$CA_CERT_NAME"; then
        create_ca
        create_secret "$CA_CERT_NAME"
    fi

    local secret
    secret="$(read_secret "$CA_CERT_NAME")"
    echo "$secret" \
        | jq --raw-output '.["tls.crt"]' \
        | base64 -d \
        > "$CA_CERT_NAME".crt
    echo "$secret" \
        | jq --raw-output '.["tls.key"]' \
        | base64 -d \
        > "$CA_CERT_NAME".key
}

# ensure_cert ensures that the cert and key for the name provided exist in the
# kubernetes API.
function ensure_cert {
    local name="${1?}"
    if ! secret_exists "$name"; then
        create_cert "$name"
        create_secret "$name"
    fi
}

function main {
    validate_env
    detect_invalid_cert_state

    ensure_ca
    IFS=';' read -ra certs <<< "$CERTS_TO_GENERATE"
    for name in "${certs[@]}"; do
        ensure_cert "$name"
    done
}
main
