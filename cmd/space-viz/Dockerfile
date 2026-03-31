FROM alpine:3.19@sha256:6baf43584bcb78f2e5847d1de515f23499913ac9f12bdf834811a3145eb11ca1
RUN apk add --no-cache ca-certificates curl
COPY space-viz /usr/local/bin/space-viz
EXPOSE 8687
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD curl -f http://localhost:8687/health || exit 1
ENTRYPOINT ["space-viz", "--port", "8687"]
