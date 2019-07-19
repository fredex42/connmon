FROM alpine:latest

COPY src/connmonn /usr/local/bin/connmonn
COPY src/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod a+x /docker-entrypoint.sh
ENTRYPOINT /docker-entrypoint.sh
