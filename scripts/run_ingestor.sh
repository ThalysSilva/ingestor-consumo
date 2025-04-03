#!/bin/bash

set -a
# Filtrar linhas: ignorar vazias e comentários, e remover espaços em branco
while IFS='=' read -r key value; do
    # Ignorar linhas vazias ou que começam com #
    if [[ -z "$key" || "$key" =~ ^\s*# ]]; then
        continue
    fi
    # Remover espaços em branco do início e do fim
    key=$(echo "$key" | xargs)
    value=$(echo "$value" | xargs)
    # Remover aspas simples ou duplas do valor, se existirem
    value=$(echo "$value" | sed -E 's/^"(.*)"$/\1/' | sed -E "s/^'(.*)'$/\1/")
    # Definir a variável de ambiente
    export "$key=$value"
done < <(grep -v '^\s*#' .env | grep -v '^\s*$')
set +a
go run cmd/ingestor/main.go