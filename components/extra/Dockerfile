FROM alpine:latest
MAINTAINER Juan M. Tirado <juan@nalej.com>

LABEL version="1.0.0"
LABEL description="Containerized ZeroTier One for use on CoreOS or other Docker-only Linux hosts."

# Uncomment to build in container
#RUN apk add --update alpine-sdk linux-headers

RUN apk add --update libgcc libstdc++ curl gnupg

RUN curl -s 'https://pgp.mit.edu/pks/lookup?op=get&search=0x1657198823E52A61' | gpg --import && \
    if z=$(curl -s 'https://install.zerotier.com/' | gpg); then echo "$z" | sh; fi


#ADD zerotier-one /
#RUN chmod 0755 /zerotier-one
#RUN ln -sf /zerotier-one /zerotier-cli
#RUN mkdir -p /var/lib/zerotier-one

#ADD main.sh /
#RUN chmod 0755 /main.sh

#ENTRYPOINT /main.sh






