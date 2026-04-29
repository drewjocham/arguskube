FROM golang:1.21-alpine AS builder

WORKDIR /app
# In a real setup, we would copy go.mod and go.sum here.
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o kubewatcher-agent main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/kubewatcher-agent .
EXPOSE 8080
CMD ["./kubewatcher-agent"]
