ARG PROJECT_NAME=exporter

FROM golang:1.26 AS build

WORKDIR /src

ARG PROJECT_NAME
ARG LDFLAGS
ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make build \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    ${LDFLAGS:+LDFLAGS="${LDFLAGS}"}

FROM debian:bookworm-slim

ARG PROJECT_NAME

COPY --from=build /src/dist/${PROJECT_NAME} /usr/local/bin/exporter

USER nobody

ENTRYPOINT ["/usr/local/bin/exporter"]