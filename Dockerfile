FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/bin/server ./main.go

FROM alpine:3.19

RUN adduser -D -H appuser

WORKDIR /app

COPY --from=builder /app/bin/server ./server

USER appuser

EXPOSE 8080

CMD ["./server"]
