FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o che-doc-generator .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl git bash \
    && curl -fsSL https://claude.ai/install.sh | bash

COPY --from=builder /app/che-doc-generator /usr/local/bin/che-doc-generator

ENTRYPOINT ["che-doc-generator"]
