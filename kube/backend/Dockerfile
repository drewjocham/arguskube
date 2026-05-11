# ---------- Build stage ----------
FROM golang:1.25-alpine AS builder

# gcc + musl-dev needed for sqlite CGo fallback and net package.
RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the headless SaaS server (no Wails GUI).
# CGO_ENABLED=1 for sqlite; static link so the binary runs on scratch/alpine.
RUN CGO_ENABLED=1 GOOS=linux \
    go build -ldflags '-s -w' \
    -o /argus-server ./cmd/server

# ---------- Runtime stage ----------
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -S kw && adduser -S kw -G kw
WORKDIR /app

COPY --from=builder /argus-server /app/argus-server

# SQLite data directory — mount a volume here for persistence.
RUN mkdir -p /data && chown kw:kw /data
ENV HOME=/data

EXPOSE 8080

USER kw
ENTRYPOINT ["/app/argus-server"]
