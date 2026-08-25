FROM golang:1.26.1-alpine3.23 AS builder

WORKDIR /app

COPY apps/server/go.mod apps/server/go.sum ./apps/server/
COPY apps/shared/go.mod ./apps/shared/

WORKDIR /app/apps/server
RUN go mod download

WORKDIR /app
COPY apps/server ./apps/server
COPY apps/shared ./apps/shared

WORKDIR /app/apps/server
RUN go build -o app ./cmd/server

FROM alpine:3.23

WORKDIR /app

COPY --from=builder /app/apps/server/app .

EXPOSE 8080

CMD ["./app"]