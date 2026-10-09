FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /modelpilot-server ./cmd/modelpilot-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && addgroup -S modelpilot && adduser -S -G modelpilot modelpilot && mkdir -p /data/jobs && chown -R modelpilot:modelpilot /data
COPY --from=build /modelpilot-server /usr/local/bin/modelpilot-server
USER modelpilot
ENV MODELPILOT_ADDR=:8080 MODELPILOT_DATA_DIR=/data/jobs
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["modelpilot-server"]
