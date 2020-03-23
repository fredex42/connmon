FROM alpine:latest

COPY src/connmonn/connmonn /usr/local/bin/connmonn
COPY src/connary/connary /usr/local/bin/connary
COPY src/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod a+x /docker-entrypoint.sh
ENTRYPOINT /docker-entrypoint.sh
