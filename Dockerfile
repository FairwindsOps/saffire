FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY kuiper .
USER nonroot:nonroot

ENTRYPOINT ["/kuiper"]
