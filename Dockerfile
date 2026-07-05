FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /synthgraph ./cmd/synthgraph

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata libc6-compat

COPY --from=builder /synthgraph /usr/local/bin/synthgraph

ENTRYPOINT ["synthgraph"]
