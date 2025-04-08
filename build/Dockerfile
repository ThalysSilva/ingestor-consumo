# Etapa de build
FROM golang:1.24-alpine AS builder

# Instala dependências necessárias para build (como git, etc.)
RUN apk add --no-cache git

WORKDIR /app

# Copia arquivos de dependência e baixa os módulos
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante da aplicação
COPY . .

# Compila o binário estaticamente para Linux
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ingestor ./cmd/ingestor/main.go

# Etapa de execução (imagem enxuta)
FROM alpine:latest

WORKDIR /root/


# Copia o binário compilado
COPY --from=builder /app/ingestor .

# Expondo a porta da aplicação
EXPOSE 8080

# Comando padrão de execução
CMD ["./ingestor"]
