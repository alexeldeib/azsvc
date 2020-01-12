#!/usr/bin/env bash
set -eux

mkdir -p ${GITHUB_WORKSPACE}/bin
cp /out/bin/kustomize ${GITHUB_WORKSPACE}/bin/kustomize
