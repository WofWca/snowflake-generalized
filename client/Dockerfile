
FROM docker.io/library/golang:1.23 AS build

ADD . /app

WORKDIR /app/client
RUN CGO_ENABLED=0 go build -o client -ldflags '-extldflags "-static" -w -s'  .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /app/client/client /bin/client

ENTRYPOINT [ "/bin/client" ]
CMD ["-broker-url=https://sf-dh-broker.duckdns.org", "-server-id=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", "-listen-address=0.0.0.0:2080" , "-max=5"]
