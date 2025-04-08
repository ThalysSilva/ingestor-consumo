#!/bin/bash

set -e  # Sai se algum comando falhar
set -a  # Exporta todas variáveis

# Lê e exporta do .env
while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" =~ ^\s*# ]] && continue
    key=$(echo "$key" | xargs)
    value=$(echo "$value" | xargs)
    value=$(echo "$value" | sed -E 's/^"(.*)"$/\1/' | sed -E "s/^'(.*)'$/\1/")
    export "$key=$value"
done < <(grep -v '^\s*#' .env | grep -v '^\s*$')

set +a

# Build e run com --env-file
docker build -f build/Dockerfile.producer -t my-producer . && \
docker run --rm --network pulse-ingestor-network --env-file .env my-producer