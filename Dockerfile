# builder
FROM golang:1.23 AS builder

WORKDIR /build

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -a -installsuffix cgo -o /app/ ./...


# runtime
FROM scratch

COPY --from=builder /app/ /app

EXPOSE 80

CMD ["/app/paste"]
