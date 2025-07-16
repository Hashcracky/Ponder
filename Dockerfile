# Build server binary and transfer it across layers
FROM golang:alpine AS build-env
RUN mkdir -p /src/app
WORKDIR /src/app
COPY pkg ./pkg
COPY go.mod ./go.mod
COPY go.sum ./go.sum
COPY main.go ./main.go
RUN go build .

# Deployment layer
FROM alpine:latest
ENV GIN_MODE=release
COPY --from=build-env /src/app/ponder /usr/local/bin/ponder

# Copy over client-side html files
RUN mkdir -p /etc/ponder/static \
    && mkdir -p /etc/ponder/static/css \
    && mkdir -p /etc/ponder/static/js \
    && mkdir -p /etc/ponder/static/img

# Copy over client-side configuration files
COPY client-side/js /etc/ponder/static/js
COPY client-side/css /etc/ponder/static/css
COPY client-side/img /etc/ponder/static/img
COPY client-side/*.html /etc/ponder/static/
COPY config/config.json /etc/ponder/config.json

# Configure non-root user
RUN addgroup --gid 10001 --system nonroot \
    && adduser  --uid 10000 --system --ingroup nonroot --home /home/nonroot nonroot \
    && apk update \
    && apk add --no-cache tini bind-tools openssl \
    && rm -rf /var/cache/apk/*

RUN mkdir -p /data/
RUN chown -R nonroot:nonroot /data/
RUN chown -R nonroot:nonroot /etc/ponder

USER nonroot
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/ponder"]
