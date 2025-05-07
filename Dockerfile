# --- Dockerfile ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git
    
WORKDIR /app
    
COPY go.mod ./
COPY . .
    
RUN CGO_ENABLED=0 go build -o lb ./cmd
    
FROM alpine:latest
    
WORKDIR /app
COPY --from=builder /app/lb .
COPY config.json .
    
EXPOSE 8080
CMD ["./lb", "--config", "config.json"]
    