FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o origin ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ffmpeg ca-certificates

WORKDIR /app

COPY --from=builder /app/origin .
COPY server.crt server.key ./

COPY uploads ./uploads
COPY hls ./hls
COPY videos.json ./

RUN mkdir -p /app/uploads /app/hls

EXPOSE 8443

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider https://localhost:8443/health || exit 1

CMD ["./origin"]