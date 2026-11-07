# dev is the Go toolchain. docker compose's api-test service runs tests and go commands in it.
FROM golang:1.27-alpine AS dev
WORKDIR /src

FROM dev AS build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM gcr.io/distroless/static-debian13
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
