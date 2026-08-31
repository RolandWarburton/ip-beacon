FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
COPY client ./client
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o beacon .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/beacon .
RUN mkdir -p data
VOLUME ["/app/data"]
EXPOSE 8080
CMD ["./beacon"]
