FROM golang:1.27.1-alpine3.24 AS build

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG BRANCH
ARG REVISION
ARG BUILD_USER
ARG BUILD_DATE
ARG BUILDER_PACKAGES="make=4.4.1-r4"

RUN apk add --no-cache ${BUILDER_PACKAGES}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make build \
    BUILD_OUTPUT=dist/prometheus-pkg-exporter \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    ${VERSION:+VERSION="${VERSION}"} \
    ${BRANCH:+BRANCH="${BRANCH}"} \
    ${REVISION:+REVISION="${REVISION}"} \
    ${BUILD_USER:+BUILD_USER="${BUILD_USER}"} \
    ${BUILD_DATE:+BUILD_DATE="${BUILD_DATE}"}

FROM alpine:3.24 AS base

ARG RUNTIME_PACKAGES="ca-certificates=20260611-r0 libcrypto3=3.5.8-r0 libssl3=3.5.8-r0"
RUN apk add --no-cache ${RUNTIME_PACKAGES}

FROM base

COPY --from=build /src/dist/prometheus-pkg-exporter /usr/local/bin/prometheus-pkg-exporter

EXPOSE 9888

USER nobody

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -T 2 --spider http://127.0.0.1:9888/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/prometheus-pkg-exporter"]
