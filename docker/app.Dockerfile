# syntax=docker/dockerfile:1

FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/gym ./cmd/gym

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/gym /usr/local/bin/gym

USER nobody

ENTRYPOINT ["gym"]
CMD ["serve"]
