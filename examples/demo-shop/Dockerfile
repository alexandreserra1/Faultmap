# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGET
RUN case "${TARGET}" in \
      checkout-service|payment-service|load-generator) ;; \
      *) echo "TARGET inválido: ${TARGET}" >&2; exit 1 ;; \
    esac \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/demo-service "./examples/demo-shop/cmd/${TARGET}"

FROM alpine:${ALPINE_VERSION}

RUN addgroup -S -g 10002 demo \
    && adduser -S -D -H -u 10002 -G demo demo

COPY --from=build /out/demo-service /usr/local/bin/demo-service

USER demo

EXPOSE 8080

ENTRYPOINT ["demo-service"]
