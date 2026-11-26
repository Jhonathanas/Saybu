# Image runtime gateway REST. Build context: backend/ (module saybu).
FROM golang:1.26.11-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/app "./services/${SERVICE}/cmd"

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S saybu \
    && adduser -S -G saybu saybu \
    && mkdir -p /srv/uploads \
    && chown saybu:saybu /srv/uploads
COPY --from=build /out/app /usr/local/bin/saybu-gateway
USER saybu
ENV TZ=Asia/Jakarta \
    APP_ENV=production
EXPOSE 8081
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8081/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/saybu-gateway"]
