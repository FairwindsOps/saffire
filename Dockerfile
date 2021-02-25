FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY saffire .
USER nonroot:nonroot

ENTRYPOINT ["/saffire"]
