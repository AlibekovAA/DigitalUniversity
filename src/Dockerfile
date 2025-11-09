FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o digitalUniversity .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/digitalUniversity .
CMD ["./digitalUniversity"]