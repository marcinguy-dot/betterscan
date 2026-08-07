FROM golang:1.26 AS build
WORKDIR /src
COPY betterscan/go.mod ./betterscan/
COPY betterscan/*.go ./betterscan/
WORKDIR /src/betterscan
RUN go build -o /out/betterscan .

FROM gcr.io/distroless/base-debian12
WORKDIR /workspace
COPY --from=build /out/betterscan /usr/local/bin/betterscan
ENTRYPOINT ["/usr/local/bin/betterscan"]
