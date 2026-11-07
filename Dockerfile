# syntax=docker/dockerfile:1

# Build the Go application.
FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod ./
COPY main.go ./
COPY static ./static

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ddos-lab .

# Run it in a small, unprivileged image.
FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app

COPY --from=builder /out/ddos-lab ./ddos-lab
COPY --from=builder /src/static ./static

USER app
EXPOSE 8080

ENTRYPOINT ["./ddos-lab"]
