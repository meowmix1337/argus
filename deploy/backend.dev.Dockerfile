FROM golang:1.26-alpine

RUN apk add --no-cache ca-certificates git

RUN go install github.com/air-verse/air@latest

WORKDIR /src

# Pre-download modules; source is bind-mounted at runtime.
COPY go.mod go.sum ./
RUN go mod download
