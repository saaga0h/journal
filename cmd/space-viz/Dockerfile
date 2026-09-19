FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
RUN apk add --no-cache ca-certificates curl
COPY space-viz /usr/local/bin/space-viz
EXPOSE 8687
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD curl -f http://localhost:8687/health || exit 1
ENTRYPOINT ["space-viz", "--port", "8687"]
