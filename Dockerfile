FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/media-gateway ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/media-gateway /app/media-gateway
COPY migrations /app/migrations

EXPOSE 8080
ENTRYPOINT ["/app/media-gateway"]
