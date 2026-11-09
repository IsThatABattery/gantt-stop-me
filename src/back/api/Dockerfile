# dev is the Go toolchain. docker compose's api-test service runs tests and go commands in it.
FROM golang:1.27-alpine AS dev
WORKDIR /src

FROM dev AS build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM build AS runner-build
RUN CGO_ENABLED=0 go build -o /runner ./cmd/runner

# runner-dev is the docker CLI plus the Go toolchain. docker compose's runner-test service runs the Docker tests in it.
FROM docker:29.9.3-cli AS runner-dev
COPY --from=dev /usr/local/go /usr/local/go
ENV GOPATH=/go PATH=/usr/local/go/bin:/go/bin:$PATH
WORKDIR /src
ENTRYPOINT []

# runner builds and runs test containers through the docker CLI. It is the only app image with Docker access.
FROM docker:29.9.3-cli AS runner
COPY --from=runner-build /runner /runner
EXPOSE 8081
ENTRYPOINT ["/runner"]

# The API stays the last stage: the api service builds the default target.
FROM gcr.io/distroless/static-debian13
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
