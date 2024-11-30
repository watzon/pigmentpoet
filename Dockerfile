FROM golang:1.23.3 AS build

WORKDIR /build/src

COPY . .

RUN mkdir -p /build/src/app

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o app/pigmentpoet .

FROM scratch

WORKDIR /usr/app

COPY --from=build /build/src/app /usr/app

# Copy ca-certificates for TLS
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

# Environment Variables:
# - BLUESKY_IDENTIFIER: Your Bluesky handle
# - BLUESKY_PASSWORD: Your Bluesky password/app password
# - TZ: Timezone for cron jobs (e.g., "America/New_York", "Europe/London", defaults to UTC)

ENTRYPOINT ["./pigmentpoet"]