# ---- Stage 1: Build frontend (Vue 3 + Vite) ----
FROM node:20-alpine AS frontend
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build
# Output: /app/uiembed/dist (vite.config.ts outDir: '../uiembed/dist')

# ---- Stage 2: Build Go binary ----
FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
# Overlay frontend build output into uiembed/dist for go:embed
COPY --from=frontend /app/uiembed/dist ./uiembed/dist/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/media-gateway ./cmd/server

# ---- Stage 3: Minimal runtime ----
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/media-gateway /app/media-gateway

EXPOSE 8080
ENTRYPOINT ["/app/media-gateway"]
