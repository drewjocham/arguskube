FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o alert-ingress ./cmd/main.go

FROM nginx:alpine
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /build/alert-ingress /usr/local/bin/alert-ingress

COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD sh -c "nginx && alert-ingress"
