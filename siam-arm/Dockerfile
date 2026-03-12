# ---------- Builder ----------
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git build-base

WORKDIR /app

# copy โครงสร้างเหมือนเครื่องจริง
COPY proto/ ./proto/
COPY worker/ ./worker/

WORKDIR /app/worker

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o siam-worker main.go

# ---------- Runtime ----------
FROM alpine:latest
RUN apk add --no-cache ca-certificates docker-cli

WORKDIR /app

# 🔥 สำคัญมาก path ต้องตรง
COPY --from=builder /app/worker/siam-worker /app/siam-worker

RUN chmod +x /app/siam-worker

ENTRYPOINT ["/app/siam-worker"]