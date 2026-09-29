FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S openflow && adduser -S -G openflow openflow
COPY --from=build /out/api /out/worker /usr/local/bin/
USER openflow
ENV OPEN_FLOW_HTTP_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["api"]

