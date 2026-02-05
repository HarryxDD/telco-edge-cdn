FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o cache-node ./cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/cache-node .

RUN mkdir -p /app/data

EXPOSE 8081

CMD ["./cache-node"]