FROM golang:1.24.11-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 64-bit Linux сборка
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./...

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/server /app/server

EXPOSE 8081
ENTRYPOINT ["/app/server"]
