# Build stage com Go 1.26.6
FROM golang:1.26.6-alpine AS builder

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

WORKDIR /

COPY --from=builder /app/main /main

RUN chmod +x /main

EXPOSE 8082

USER 65532:65532

LABEL org.opencontainers.image.title="beer-avaliation-api" \
      org.opencontainers.image.description="Beer catalog API with moderation" \
      org.opencontainers.image.vendor="Beer Review" \
      org.opencontainers.image.licenses="MIT"

CMD ["/main"]
