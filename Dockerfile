FROM docker.io/library/golang:1-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Run tests before building
RUN go test ./...
# Build with metadata (static binary)
ARG BUILD_TIME
ARG GIT_COMMIT
ENV CGO_ENABLED=0
RUN go build -ldflags "-X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" -o dead-mans-switch .

FROM docker.io/library/alpine:3
WORKDIR /app
COPY --from=build /app/dead-mans-switch .
COPY --from=build /app/web ./web
CMD ["./dead-mans-switch"]
