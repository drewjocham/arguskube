FROM golang:1.25-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -ldflags '-s -w' -o /flink-gateway .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    addgroup -S kw && adduser -S kw -G kw
COPY --from=build /flink-gateway /flink-gateway
USER kw
EXPOSE 8087
ENTRYPOINT ["/flink-gateway"]
