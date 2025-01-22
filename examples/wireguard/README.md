# WireGuard example

<!-- TODO maybe we should generalize this instruction to
OpenVPN as well? To use with AmneziaVPN.
Or is this guide for non-Amnezia clients, so that VPN providers are happy? -->

This example sets up a WireGuard server
that is accessible through a Snowflake tunnel.
The client can connect to the server using the official WireGuard client
and use it as a system-wide VPN.

## ⚠️ Not production-ready

This setup is not production-ready, due to the following issues:

- The VPN client might stop working after a while, requiring a restart.

    If a Snowflake proxy disconnects from you,
    your Snowflake client won't be able to find a new proxy,
    which would result in the connection failing until you restart it,
    because the Snowflake client would try to connect to the broker
    _through the WireGuard tunnel_, which would obviously be down.

    You might think that this can be fixed by setting
    [`-max=2`](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/e4c95fc2424f343a808a0ae93a0becc2b1b9c023/client/snowflake.go#L174-175)
    to be connected to two (or more) Snowflake proxies at the same time,
    but no: if a proxy goes down,
    the client would, again, try to connect to the new proxy through
    the WireGuard tunnel, which would _not_ be down this time,
    but it still doesn't make sense.
    See [relevant comment](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/25723#note_3076796)
    in upstream Snowflake.

    A real solution would be to utilize split-tunneling
    to make the Snowflake client bypass the VPN tunnel
    (this feature is present in the Amnezia VPN client, by the way),
    or to utilize the
    ["bind to all interfaces" IP handling policy](https://github.com/pion/ice/issues/750),
    but it's not yet supported by Pion (Snowflake's WebRTC library).
    Perhaps together with Pion's `SetInterfaceFilter()`
    to exclude the VPN's interface.
- The connection is terribly slow (expect ~0.5 Mbit / s).

    Apart from general slow-ness of snowflake (see e.g.
    [this issue](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40026)
    and the "Not as fast as it can be" section at the root README),
    I suspect that the current implementation of the UDP mode in this project
    is not able to preserve UDP packets in their "datagram" form
    and simply turns them into a stream of data,
    so sometimes two input packets at the client side get mushed together
    into one at the server side,
    and sometimes one packet gets split into two.
    And WireGuard is not able to make sense
    of such packets and simply throws them away.
    But this is just a guess.

    Another possible explanation is something akin to TCP meltdown:
    WireGuard is UDP-based, so it has its own reliability layer,
    but the Snowflake channel is also reliable
    (see a note about KCP in [the Snowflake paper](https://www.bamsoftware.com/papers/snowflake/)),
    so they don't go well together.
    This could be solved by introducing unreliable mode to Snowflake.
    Merging ["Unreliable+unordered WebRTC data channel transport"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/315)
    in the upstream Snowflake would be a step in this direction.

So, take this example as just a showcase of what's possible for now.

## Instructions

We're gonna use [Amnezia VPN](https://amnezia.org/downloads)
to set up the server, because it's easy.
However, it doesn't matter what method we use,
as long as we get a working WireGuard server at the end.

### Server setup

WireGuard server setup:

1. Obtain a server (e.g. VPS) with a public IP.
1. Install [Amnezia VPN](https://amnezia.org/downloads) on your personal device.
1. Add your server to Amnezia VPN: enter its IP and password.
1. Through Amnezia VPN UI, set up WireGuard on your server.
<!-- 1. Generate a WireGuard config through the "share" function in Amnezia VPN.
    This config will be used in the client setup. -->

Snowflake server setup:

1. Obtain a domain name.
    You can get one easily and for free on <https://www.duckdns.org/>.
1. Point the domain name to your server's IP.
1. Download this project's code.

    ```bash
    git clone https://github.com/WofWca/snowflake-generalized.git \
        && cd snowflake-generalized
    ```

1. Make sure Docker is installed, with `docker --version`.
    If not, [install Docker](https://docs.docker.com/engine/install/).
1. In Amnezia VPN, check the port of the WireGuard server:
    you'll need it in the next step.
1. Launch the server with Docker:

    ```bash
    docker build \
        -t snowflake-generalized-server \
        --file ./server/Dockerfile . \
        && docker run snowflake-generalized-server \
        -destination-address=localhost:<YOUR_WIREGUARD_SERVER_PORT> \
        -listen-address=:7901 \
        -acme-hostnames=<YOUR_DOMAIN_NAME> \
        -destination-protocol=udp \
        -acme-cert-cache=/acme-cert-cache
    ```

    Replace `<YOUR_WIREGUARD_SERVER_PORT>`
    and `<YOUR_DOMAIN_NAME>` accordingly.

### Client setup

1. [Install WireGuard client](https://www.wireguard.com/install/).
1. [Install Go](https://go.dev/doc/install).
1. Download this project's code.

    ```bash
    git clone https://github.com/WofWca/snowflake-generalized.git \
        && cd snowflake-generalized
    ```

1. Start the client:

    ```bash
    go run ./client \
        --broker-url=https://<BROKER_DOMAIN_NAME> \
        --server-url=wss://<YOUR_SERVER_DOMAIN_NAME>:7901 \
        --listen-address=127.0.0.1:51821 \
        --destination-protocol=udp \
        --keep-local-addresses
    ```

    Replace the parts with brackets accordingly.  
    (`--keep-local-addresses` seems to be required,
    otherwise the proxy connection gets interrupted
    as soon as you enable the WireGuard tunnel,
    but I am not quite sure why).

1. Obtain a WireGuard client config file.
    In our example you can do it through the Amnezia UI,
    using the "share" function.
1. Open the WireGuard config file with a text editor and,
    in the `Endpoint=` field, replace the IP address
    of the WireGiard server (e.g. `123.1.12.123`) with `127.0.0.1`,
    and port (the part after the colon (`:`)) with `51821`.
1. Import the config file into the WireGuard client.
1. In the WireGuard UI, make sure to disable
    "Block untunneled traffic (kill-switch)".
    This is needed for the Snowflake client to be able to communicate
    with the Snowflake proxy directly and not through the WireGuard tunnel.
1. Press "Activate".

You're now using WireGuard through a Snowflake tunnel!

## Addendum

Setting up OpenVPN or any other VPN client should be just as easy,
as long as you disable the "kill-switch" feature,
or utilize the "split-tunneling feature" (if the VPN client has one) and
add the snowflake client executable file to the white-list
(so that it bypasses the VPN).
Amnezia VPN is one such client that has the split-tunneling feature.

Also consider utilizing the `-single-connection-mode` flag
on both the server and the client
to see if it improves performance,
though as of 2025-01 this doesn't appear to.
