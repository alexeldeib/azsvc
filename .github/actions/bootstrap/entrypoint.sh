#!/usr/bin/env bash
set -eux

mkdir -p ${GITHUB_WORKSPACE}/bin
cp /out/bin/bootstrap ${GITHUB_WORKSPACE}/bin/bootstrap

/out/bin/bootstrap -f "${FILE}" --app "${AZURE_CLIENT_ID}"--tenant "${AZURE_TENANT_ID}"--key "${AZURE_CLIENT_SECRET}"
