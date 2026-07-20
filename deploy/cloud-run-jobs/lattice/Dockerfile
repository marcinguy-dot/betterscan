FROM golang:1.23 AS build
WORKDIR /src
COPY lattice/go.mod ./lattice/
COPY lattice/*.go ./lattice/
WORKDIR /src/lattice
RUN go build -o /out/lattice .

FROM gcr.io/distroless/base-debian12
WORKDIR /workspace
COPY --from=build /out/lattice /usr/local/bin/lattice
ENTRYPOINT ["/usr/local/bin/lattice"]
