FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o cortex-ia ./cmd/cortex-ia

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl nodejs npm \
    && addgroup -S cortexia && adduser -S cortexia -G cortexia

COPY --from=builder /build/cortex-ia /usr/local/bin/cortex-ia

USER cortexia

ENTRYPOINT ["cortex-ia"]
CMD ["help"]
