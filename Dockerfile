# syntax=docker.io/docker/dockerfile:1

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/soyacen/envmerge/internal/cli.Version=${VERSION}" \
    -o /out/envmerge ./cmd/envmerge

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/envmerge /usr/local/bin/envmerge
ENTRYPOINT ["/usr/local/bin/envmerge"]
