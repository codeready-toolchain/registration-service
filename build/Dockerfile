FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

LABEL maintainer "Devtools <devtools@redhat.com>"
LABEL author "Devtools <devtools@redhat.com>"

ENV REG_SERVICE=/usr/local/bin/registration-service \
    USER_UID=1001 \
    USER_NAME=registration-service \
    LANG=en_US.utf8

# install registration-service binary
COPY build/_output/bin/registration-service ${REG_SERVICE}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/registration-service"]

EXPOSE 8080

USER ${USER_UID}
