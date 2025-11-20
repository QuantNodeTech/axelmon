FROM golang:alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o axelmon .

FROM alpine
WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/axelmon /usr/local/bin/axelmon
COPY config.toml.example /app/config.toml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/axelmon"]
CMD ["-config", "/app/config.toml"]
