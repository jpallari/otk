FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux pipefail && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        git openssh-server curl

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/usr/sbin/sshd", "-D" ]
