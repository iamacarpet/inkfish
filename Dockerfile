FROM golang:1.11 AS build-env
RUN useradd -u 10001 inkfish
WORKDIR /src

COPY go.mod go.sum /src/
RUN go mod download
ADD . /src
RUN make

FROM bitnami/minideb:stretch
WORKDIR /app
COPY --from=build-env /src/testdata/demo_config /config/demo
COPY --from=build-env /src/build/inkfish-linux /inkfish
USER inkfish

CMD ["/inkfish"]
