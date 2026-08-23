ARG PROJECT_NAME=exporter

FROM golang:1.27.0 AS build

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

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=build /src/dist/${PROJECT_NAME} /usr/local/bin/exporter

EXPOSE 9888

USER nobody

ENTRYPOINT ["/usr/local/bin/exporter"]
