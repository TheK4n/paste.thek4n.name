# builder
FROM golang:1.23 AS builder

WORKDIR /build

ARG APP_VERSION
ENV APP_VERSION=$APP_VERSION

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    OUTPUTDIR="/app/" APP_VERSION=$APP_VERSION ./bin/make build


# upx
FROM ubuntu:22.04 AS upx

RUN apt-get update -y && apt-get install -y --no-install-recommends upx

COPY --from=builder /app/ /app

RUN upx --best --no-lzma /app/*


# runtime
FROM scratch

COPY --from=upx /app/ /app

EXPOSE 80

CMD ["/app/paste", "--host", "0.0.0.0", "--port", "80", "--dbport", "6379", "--health"]
