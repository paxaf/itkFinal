FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/main ./cmd
RUN CGO_ENABLED=0 GOBIN=/out \
    go install github.com/pressly/goose/v3/cmd/goose@v3.26.0

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/main ./main
COPY --from=builder /out/goose /usr/local/bin/goose
COPY config.env ./config.env
COPY migrations ./migrations
COPY entrypoint.sh ./entrypoint.sh

RUN chmod +x ./entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
CMD ["./main", "-c", "config.env"]