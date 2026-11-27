FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/private-platform-api ./cmd/api

FROM alpine:3.21
RUN addgroup -S -g 10001 app && adduser -S -u 10001 -G app app
COPY --from=build /out/private-platform-api /usr/local/bin/private-platform-api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/private-platform-api"]
