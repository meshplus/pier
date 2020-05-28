#!/bin/bash
set -e

APPCHAIN_NAME=$1
PLUGIN_CONFIG=$2

echo $APPCHAIN_NAME

pier --repo=/root/.pier appchain register --name=${APPCHAIN_NAME} --type=fabric --validators=/root/.pier/${PLUGIN_PATH}/fabric.validators --desc="appchain for test" --version=1.4.1
pier --repo=/root/.pier rule deploy --path=/root/.pier/validating.wasm
pier --repo=/root/.pier start

exec "$@"