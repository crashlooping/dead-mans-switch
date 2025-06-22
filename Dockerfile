FROM docker.io/library/golang:1-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o dead-mans-switch .

FROM docker.io/library/alpine:3
WORKDIR /app
COPY --from=build /app/dead-mans-switch .
COPY --from=build /app/web ./web
CMD ["./dead-mans-switch"]
