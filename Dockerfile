FROM golang:1.12.9 AS build-env

WORKDIR /go/src/github.com/codeready-toolchain/registration-service

ENV GO111MODULE=on

COPY . /go/src/github.com/codeready-toolchain/registration-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o metrics

FROM alpine
LABEL maintainer "Michael Kleinhenz <kleinhenz@redhat.com>" \
      author "Michael Kleinhenz <kleinhenz@redhat.com>"

RUN apk --no-cache add ca-certificates

EXPOSE 8080

COPY --from=build-env /go/src/github.com/codeready-toolchain/registration-service/registration-service /usr/local/bin/
USER 10001

ENTRYPOINT [ "/usr/local/bin/registration-service" ]
