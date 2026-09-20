FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /taskotter ./cmd/taskotter-sync


FROM alpine:3.24

RUN apk add --no-cache ca-certificates git

COPY --from=builder /taskotter /taskotter

ENTRYPOINT ["/taskotter"]