FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL maintainer "Devtools <devtools@redhat.com>"
LABEL author "Devtools <devtools@redhat.com>"

ENV REG_SERVICE=/usr/local/bin/registration-service \
    USER_UID=1001 \
    USER_NAME=registration-service \
    LANG=en_US.utf8

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# install registration-service binary
COPY build/_output/bin/registration-service ${REG_SERVICE}

ENTRYPOINT ["/usr/local/bin/registration-service"]

EXPOSE 8080 8081 8082

USER ${USER_UID}
