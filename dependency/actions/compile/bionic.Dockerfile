FROM ubuntu:18.04

RUN apt-get -y update
RUN apt-get -y install curl tar

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]