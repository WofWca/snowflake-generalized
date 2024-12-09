# Used as a reference:
# https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/main/Dockerfile

FROM docker.io/library/golang:1.23 AS build

# # https://docs.podman.io/en/latest/markdown/podman-auto-update.1.html
# LABEL io.containers.autoupdate=registry
# LABEL org.opencontainers.image.authors="anti-censorship-team@lists.torproject.org"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

WORKDIR /app/server
RUN CGO_ENABLED=0 go build -o server -ldflags '-extldflags "-static" -w -s' .

RUN mkdir /acme-cert-cache

RUN groupadd nonroot
RUN useradd --gid nonroot nonroot

FROM scratch

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
USER nonroot:nonroot

COPY --from=build --chown=nonroot:nonroot /acme-cert-cache /acme-cert-cache
VOLUME [ "/acme-cert-cache" ]

# Do we need certificates? They might be required for Let's Encrypt.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /app/server/server /bin/server

ENTRYPOINT [ "/bin/server" ]
