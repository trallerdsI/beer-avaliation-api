# Build stage com Go 1.26.5
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

# Copia go.mod e go.sum primeiro para aproveitar o cache de módulos
COPY go.mod go.sum ./
RUN go mod download

# Copia o código-fonte
COPY . .

# Build estático (CGO desabilitado) para rodar em distroless
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main cmd/server/main.go

# Imagem final mínima e segura (sem shell, sem pacotes extras)
FROM gcr.io/distroless/static-debian12

WORKDIR /root/

COPY --from=builder /app/main /root/main
COPY --from=builder /app/migrations/ /root/migrations/

# Porta exposta (mantém 8082 para alinhar com SERVER_PORT padrão)
EXPOSE 8082

USER nonroot:nonroot

CMD ["/root/main"]
