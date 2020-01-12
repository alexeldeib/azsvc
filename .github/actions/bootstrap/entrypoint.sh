#!/usr/bin/env bash
set -eux

mkdir -p ${GITHUB_WORKSPACE}/bin
cp /out/bin/bootstrap ${GITHUB_WORKSPACE}/bin/bootstrap
