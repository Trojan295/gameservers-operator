FROM alpine:3.10

ENV OPERATOR=/usr/local/bin/gameservers-operator \
    USER_UID=1001 \
    USER_NAME=gameservers-operator

# install operator binary
COPY build/_output/bin/gameservers-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
