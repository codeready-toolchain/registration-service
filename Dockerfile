FROM golang:1.12.9 AS build-env

WORKDIR /go/src/github.com/codeready-toolchain/registration-service

ENV GO111MODULE=on

COPY . /go/src/github.com/codeready-toolchain/registration-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

FROM alpine
LABEL maintainer "Michael Kleinhenz <kleinhenz@redhat.com>" \
    author "Michael Kleinhenz <kleinhenz@redhat.com>"

RUN apk --no-cache add ca-certificates

EXPOSE 8000
COPY --from=build-env /go/src/github.com/codeready-toolchain/registration-service/registration-service /usr/local/bin/

# Fixes this issue:
# https://stackoverflow.com/questions/34729748/installed-go-binary-not-found-in-path-on-alpine-linux-docker
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

ENTRYPOINT [ "/usr/local/bin/registration-service", "--port=8000", "--insecure" ]
