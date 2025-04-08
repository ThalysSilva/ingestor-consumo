# Carrega e exporta variáveis do .env para o ambiente (útil se for usar no host também)
Get-Content .env | ForEach-Object {
    if ($_ -match "^\s*([^#\s=]+)\s*=\s*(.*?)\s*$") {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim() -replace '^"(.*)"$', '$1' -replace "^'(.*)'$", '$1'
        [System.Environment]::SetEnvironmentVariable($name, $value)
    }
}

# Build e run usando --env-file
docker build -f build/Dockerfile.producer -t my-producer .; if ($?) {
    docker run --rm --network pulse-ingestor-network --env-file .env my-producer
}