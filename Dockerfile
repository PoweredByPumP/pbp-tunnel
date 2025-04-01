FROM alpine:3.21.3

ARG ARTIFACT_NAME="pbp-tunnel"

COPY ./$ARTIFACT_NAME /usr/local/bin/pbp-tunnel
RUN chmod +x /usr/local/bin/pbp-tunnel

ENTRYPOINT ["pbp-tunnel"]
