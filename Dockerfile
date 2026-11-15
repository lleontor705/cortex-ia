# Stage 1: Build binary
FROM golang:alpine AS builder
WORKDIR /app
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cortex-report-hub ./cmd/cortex-report-hub

# Stage 2: Minimal runtime image
FROM alpine:3.21
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /cortex-report-hub /app/cortex-report-hub
ENV PORT=8080
ENV DATA_DIR=/app
EXPOSE 8080
CMD ["/app/cortex-report-hub"]
