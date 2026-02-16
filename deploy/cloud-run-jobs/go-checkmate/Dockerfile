FROM golang:1.23 AS build
WORKDIR /src
COPY go-checkmate/go.mod ./go-checkmate/
COPY go-checkmate/*.go ./go-checkmate/
WORKDIR /src/go-checkmate
RUN go build -o /out/checkmate-go .

FROM gcr.io/distroless/base-debian12
WORKDIR /workspace
COPY --from=build /out/checkmate-go /usr/local/bin/checkmate-go
ENTRYPOINT ["/usr/local/bin/checkmate-go"]
