# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notification-service .

FROM scratch
COPY --from=builder /out/notification-service /notification-service
EXPOSE 8082
ENTRYPOINT ["/notification-service"]
