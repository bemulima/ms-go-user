ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o user-service ./cmd/user-service \
    && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o user-service-migrate ./cmd/user-service-migrate

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/user-service /app/user-service
COPY --from=builder /app/user-service-migrate /app/user-service-migrate
COPY --from=builder /app/migrations /app/migrations
COPY .env.example /app/.env
EXPOSE 8080
ENTRYPOINT ["/app/user-service"]
