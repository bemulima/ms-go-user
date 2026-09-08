ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o user-service ./cmd/user-service

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/user-service /app/user-service
COPY .env.example /app/.env
EXPOSE 8080
ENTRYPOINT ["/app/user-service"]
