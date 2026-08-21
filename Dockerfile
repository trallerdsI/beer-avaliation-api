# Build stage otimizado com cache de módulos via BuildKit
FROM golang:1.26.6-alpine AS builder

# Instala git e ca-certificates necessários para go mod download
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache de dependências: go.mod/go.sum são copiados primeiro
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Copia o resto do código-fonte
COPY . .

# Build otimizado: estático, sem símbolos de debug, sem paths absolutos
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildvcs=false -X main.Version=${VERSION}" \
    -o main ./cmd/server

# Imagem final mínima: distroless estático sem shell, sem libs extras
FROM gcr.io/distroless/static-debian12

WORKDIR /

# Copia apenas o binário otimizado do stage anterior
COPY --from=builder /app/main /main

# Executa como usuário não-root (65532 é o usuário padrão do distroless)
USER 65532:65532

EXPOSE 8082

# Health check: endpoint /api/v1/health retorna 200 quando pronto
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/main", "health"] || exit 1

CMD ["/main"]
