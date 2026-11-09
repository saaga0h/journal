FROM alpine:3.23@sha256:469c9ea410b9215eaee7976ba6c0639039b444be8e5d638f7abbada189971eb2
RUN apk add --no-cache ca-certificates curl
COPY space-viz /usr/local/bin/space-viz
EXPOSE 8687
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD curl -f http://localhost:8687/health || exit 1
ENTRYPOINT ["space-viz", "--port", "8687"]
