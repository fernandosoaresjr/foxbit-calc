# syntax=docker/dockerfile:1

# --- Estágio de build -------------------------------------------------------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Baixa dependências primeiro (camada cacheável enquanto go.mod/go.sum não mudam).
COPY go.mod go.sum ./
RUN go mod download

# Copia o código e compila um binário estático (CGO desabilitado).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- Imagem final -----------------------------------------------------------
# distroless/static:nonroot: sem shell, sem package manager, usuário não-root
# (uid 65532) — superfície de ataque mínima.
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/server /server

EXPOSE 8000
USER nonroot:nonroot

ENTRYPOINT ["/server"]
