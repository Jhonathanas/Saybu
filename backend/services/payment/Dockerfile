# Image runtime payment service. Build context: backend/ (module saybu).
FROM golang:1.26.11-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=payment
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/app "./services/${SERVICE}/cmd"

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S saybu \
    && adduser -S -G saybu saybu
COPY --from=build /out/app /usr/local/bin/saybu-payment
USER saybu
ENV TZ=Asia/Jakarta \
    APP_ENV=production
EXPOSE 50055 9105
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:9105/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/saybu-payment"]
