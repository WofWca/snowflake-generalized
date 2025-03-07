# ❄️ Snowflake Generalized

Censorship circumvention software like
[Snowflake](https://snowflake.torproject.org/),
but faster thanks to not using Tor.

Acts as a TCP or UDP tunnel
between the user ([client](./client/main.go)) and
the [server](./server/main.go),
much like `ssh -L`.
<!-- As proposed in
["Snowflake as a generic TCP (UDP?) forwarder (like `ssh -L`)"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40168) -->

This, in turn, allows the client to access any blocked TCP / UDP service,
such as [a SOCKS proxy](https://github.com/serjs/socks5-server)
or your favorite (but blocked) VPN service provider.

<!-- FYI we also talk about the fork's changes below. -->
This project is based on a _fork_ of Snowflake.
The difference between the fork and the original can be found
[here](https://gitlab.torproject.org/WofWca/snowflake/-/compare/main...for-snowflake-generalized?from_project_id=43).
In summary, the changes are:

- Allow clients to ask proxies to connect to any host they choose.
- Add a lot of hardening features
    (which are needed because of the previous bullet point).
    See ["Security"](#security).
- Add WASM support for the proxy
    (see [the relevant MR](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/513)).

## Try it

<!-- FYI this section is linked from ./examples/socks-server/README.md,
and in this README -->

1. [Install Go](https://go.dev/doc/install).
2. Clone the code:

    ```bash
    git clone https://github.com/WofWca/snowflake-generalized.git \
        && cd snowflake-generalized
    ```

3. Launch the client:

    ```bash
    go run ./client \
        -broker-url=https://sf-dh-broker.duckdns.org/ \
        -server-url=wss://envoy1-snowflake.ce.unredacted.org:7901 \
        -listen-address=localhost:2080 \
        -destination-protocol=tcp
    ```

4. Open another terminal and access a website through the Snowflake tunnel:

    ```bash
    curl --proxy "socks5://localhost:2080" https://api.ipify.org
    ```

Now you can set your browser to use the SOCKS5 proxy at `localhost:2080`.

For more info about how this example works,
see [./examples/wireguard/](./examples/wireguard/).

Many thanks to [Unredacted.org](https://unredacted.org/)
for setting up a public SOCKS server!

## Background knowledge

[Snowflake](https://snowflake.torproject.org/) is developed by The Tor Project.
As of 2024, it can only be used to access the Tor network.

A Snowflake client is embedded into Tor Browser and [Orbot](https://orbot.app/).
Orbot provides VPN-like functionality by tunneling traffic through Tor,
optionally accessing Tor through bridges, such as Snowflake.

![Architecture of Snowflake-generalized](./docs/snowflake-generalized.svg)
(An altered figure from
[the Snowflake paper](https://www.bamsoftware.com/papers/snowflake/)
(which you should check out (it's not that hard to read)))

But, as you might imagine,
making your traffic bounce off of 1 + 3 relays
(extra 1 is a Snowflake proxy) before it reaches the destination
could be quite slow.  
If you're only looking to browse Instagram or YouTube
or whatever else is blocked for you,
and you
[don't care](https://en.wikipedia.org/wiki/Cute_cat_theory_of_digital_activism)
about anonymity that Tor provides, you'd want Snowflake to connect directly
(or at least _more_ directly)
to where you want to go, and not through a huge chain of relays.

Luckily, as it turns out,
Tor is not inherent in Snowflake.
And this is where our project comes in!
<!-- Moreover, Snowflake isn't really concerned about
what kind of traffic you pass through it! -->

## How it works

Pretty much like Snowflake.
In fact you can argue that this project is just Snowflake,
but with adjusted arguments.  
The source code is mostly boilerplate.

## Is it production-ready?

Not yet.

### Not as fast as it can be (but still faster!)

<!-- FYI this section is linked to in ./examples/wireguard/README.md -->

Upstream Snowflake is still pending quite a lot of work
to improve its performance (download speed and latency).  
But at the current state of the project, _20+ Mbit / s_
with download latency below 700 ms is already achievable!

For reference, here are some other performance-related issues,
which also include performance measurements:

- [Use unreliable and unordered WebRTC data channels](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40352)
- [Tune reliable protocol parameters for Snowflake](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40026)
- [Multiplex - one client splits traffic across multiple proxies](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/25723)

### Easy to block

The real power of a Snowflake network comes from
the numerosity of its proxies.
Ideally we'd want one huge Snowflake network
(which the original one from The Tor Project is)
where proxies are happy to pass client's traffic
wherever the client wishes,
without making the proxy operator install
an extra browser extension or a Docker container per each
server that they want to let clients get unrestricted access to.  
But, as was said, by default the proxies can only forward traffic to
just a few set-in-stone Tor relays.

In order to change that and
[let proxies connect to arbitrary addresses](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40248),
we need to make a lot of hardening changes
to the proxy's code such that they cannot be abused,
e.g. for DDoS,
access to the proxy operator's private network,
or distribution of illegal content.  
And I am trying to do just that with
[my recent MRs](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests?scope=all&state=all&author_username=WofWca).
<!-- FYI we also talk about the fork above -->

## Security

<!-- FYI this section has links to it in this README. -->
So, the proxy
[can connect to arbitrary addresses, specified by clients](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/379).
This begs the question:
Is it even safe to run a proxy on a VPS, let alone from home?

To address this, we have several security layers:

<!-- TODO explain what each of these protects against?
Or is what we have sufficient already? -->
- The proxy will only accept to connect to _one single_, somewhat obscure, port.

    We chose port `7901`.

    It is not very likely (but still possible)
    that any services are running on this port,
    except a Snowflake server.
    So if there is no application listening on this port,
    the target host should simply reject such a connection,
    and nothing should happen.

    If the client specifies a different port,
    the proxy will reject such a request,
    and no network activity will occur at all,
    not even DNS
    (apart, of course, from sending an error response to the broker).

    See [the relevant MR, "hardening(proxy): check port against relay pattern"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/381).

    (Just FYI this port is already assigned by IANA
    to another application, "TNOS Service Protocol".
    See the [IANA Port Number Registry](https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xhtml?search=7901)).
- The proxy will not connect to _private_ IP addresses.

  - Standalone (native) proxy:

      The standalone (native) proxy will perform a DNS request,
      and verify that the target host does not resolve
      to any private addresses.
      If the host is at a private address,
      the proxy will reject the client's request,
      and will not send any packets to the target host.

      See [the relevant commit, "hardening(proxy): `!allowPrivateIPs`: perform DNS"](https://gitlab.torproject.org/WofWca/snowflake/-/commit/2438ec9e7ca00a4290600ed40527bd0229428cd3).
  - Browser-based proxy:

      The DNS check, however, is not always available
      in the browser extension version of the proxy.
      It requires the DNS permission, which Chromium does not support,
      as of 2025-03.
      See [`dns.resolve()`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/dns/resolve).

      In browsers we will rely on the concept called
      ["Private Network Access" (PNA)](https://developer.chrome.com/blog/private-network-access-preflight/).
      When the browser determines that the target server is located
      on a private address (such as `127.0.0.1` or `192.168.1.1`),
      it will perform a preflight request to the server,
      and expect it to explicitly specify in the response headers
      that it allows public origins to access it
      (`Access-Control-Allow-Private-Network: true`).
      If this is not the case, the proxy will refuse to proceed.
      "Private Network Access" is not yet fully adopted by browsers,
      but we are prepared for it: we set the `treat-as-public-address`
      CSP directive.

      Note that, at the current state, PNA is more invasive,
      because it does perform an HTTP request to the target server,
      unlike a simple DNS request.

  In addition, if the client tried to specify the target host by IP,
  the proxy won't even have to perform the DNS request:
  determining whether an IP is private requires no network activity.
  Note that, contrary to what one might think,
  private machines _may_ and often do have a host name associated with them,
  for example `myrouter.local`
  (this is [used e.g. by Linksys](https://support.linksys.com/kb/article/378-en/)).
- The _broker_ will reject the client's request if the target host
    doesn't resolve to a public IP address.

    The broker will perform its own DNS request.

    This alone, of course, does not save the proxy if the broker goes evil,
    but it's still a layer of security.

    However, as was mentioned, a domain name might resolve to a public address
    on the broker side, but to a public address on the proxy side.
    For example, ASUS routers can be accessed from the home network
    by domain name `www.asusrouter.com`
    (see [their help article on this](https://www.asus.com/support/faq/1005263/)).
    Even though the domain name doesn't resolve to a public address
    as of 2025-02, they might change their mind about it.

    See [the relevant commit, "hardening(broker): DNS check if relayURL is public"](https://gitlab.torproject.org/WofWca/snowflake/-/commit/7d53658f83638a476a117b42fa733b83d53413f5).
- The proxy will only accept to connect to _TLS_ (HTTPS) servers
    (and not unencrypted bare HTTP servers).

    Most of the time (but not always!),
    HTTP servers that run on private addresses
    do not utilize TLS.
    If the target server doesn't speak TLS, the connection to it will fail.
    The target server will still receive packets from the proxy,
    but it's basically gonna looks like garbage to it.

    To the best of my knowledge, at this stage
    (during TLS connection establishment)
    the client still cannot in any way control
    the contents of the packets that the proxy sends to the target host.

    Note that some routers, such as Linksys
    (see [this help article](https://support.linksys.com/kb/article/378-en/))
    _do_ utilize TLS (`https://192.168.1.1`).
- The proxy will check if the target HTTP server _is a Snowflake server_.

    The proxy will send a benign HTTP OPTIONS request to the target server,
    with a special header (`Are-You-A-Snowflake-Server`).
    The server must respond with another header (`I-Am-A-Snowflake-Server`),
    otherwise the proxy will refuse to proceed.
    The contents of such a request, apart from the host address,
    are not controlled by the client.
    This is similar to the existing concept called
    ["preflight requests"](https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request).

    See [the relevant MR, "hardening: make proxies request server consent"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/385).
- The proxy will only do _WebSocket_ connections (later WebTransport).

    If the target server is not a WebSocket server,
    the connection will fail.

    This is inherent in the WebSocket protocol.
    See [The WebSocket Protocol RFC: Opening Handshake](https://www.rfc-editor.org/rfc/rfc6455.html#section-1.3).
- The proxy will check if the WebSocket server claims to support
    the Snowflake-specific subprotocol.

    In some cases the WebSocket connection will fail at hanshake level
    (e.g. in Chromium).
    If that didn't happen, we'll explicitly verify
    that the WebSocket server picked the right protocol name string.

    See [the relevant commit, "hardening: RequireWebSocketSubprotocolNegotiation"](https://gitlab.torproject.org/WofWca/snowflake/-/commit/0e164a6119aac01b2543a9c78f3bc0192110a599).
- As an extra layer to ensure some of the above claims,
    the browser extension version of the proxy utilizes
    [Content Security Policy (CSP)](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP).
    Namely, the CSP includes the following:

    ```txt
    connect-src
        wss://*:7901
        https://*:7901/are_you_a_snowflake_server
        https://<BROKER_DOMAIN_NAME>
        https://snowflake-broker.torproject.net:8443/probe
    ```

- Side-channel attack protection.

    For the majority of the abovementioned checks, if a check fails,
    the proxy will not reveal any information to the client
    about the kind of error that occured.
    This includes timing attack protection: the proxy will respond
    to the client after a fixed amount of time, should an error occur.

TODO (unimplemented) extra measures:

- Utilize WebSocket subprotocol negotiation.
- Respect the `Access-Control-Allow-Origin` response header.
- Make the broker perform the consent request.
- Make the broker only accept servers which registered on this broker.

Please let me know if you have ideas / concerns!

## Can I run a proxy?

Yes, you can help this project help others circumvent censorship.  
However, remember:

- This project is still work-in-progress.
    But we'll continue improving things,
    so I guess you could just launch the proxy and forget about it:
    it should update itself automatically, if you use the command below.
- It might not be safe to run a proxy on a personal machine and+or network.  
    Do _I_ think it's safe? Yes, it looks pretty safe to me.
    But, AFAIK, nobody else has assessed the security of my changes.

If you want to proceed:

1. [Install Docker](https://docs.docker.com/engine/install/).
2. Run

    ```bash
    curl -O https://gitlab.torproject.org/WofWca/snowflake/-/raw/for-snowflake-generalized/docker-compose-proxy.yml \
        && docker compose -f docker-compose-proxy.yml up --detach
    ```

Again, the proxy code is based on the original Snowflake code,
so you could also find
[Tor Project's instructions](https://community.torproject.org/relay/setup/snowflake/standalone/docker/)
and
[Tor Project Forum](https://forum.torproject.org/tag/snowflake)
useful.

## Why did you make the project?

My dream is: Snowflake clients can access _any_ service they want
with the help of Snowflake proxies,
without having to route the traffic through the (not the fastest) Tor network,
and the Snowflake proxies don't have to worry about being abused,
and that all VPN providers have a Snowflake server running,
ready to accept clients for whom their service is blocked.

Snowflake is a fascinating concept and I think it has a lot of potential.

## Usage

For a quick demo, see [the "Try it" section](#try-it).

Also see [./examples](./examples)

***

This guide will walk you through setting up _an entire_ Snowflake network,
with all 4 of its components.  
However, as a regular user, or a service provider,
you'll only need to set up one:
users will need to set up the client,
and service providers will need to set up the server.
You're supposed to be using the broker and the proxies that other people
already set up publicly.

With that out of the way,
let's make a setup that will work in the same way as

```bash
ssh -L localhost:2080:example.com:80 my-server-1.my-domain.com -N
```

That is, if you connect with TCP to `localhost:2080` on your local machine,
the connection will be performed to `example.com:80` from
`my-server-1.my-domain.com`.

> A Snowflake network consists of components of 4 types.
> In production, each component usually runs on a different machine,
> but for testing purposes you can run all of them locally.

### Prerequisites

- [Install Go](https://go.dev/doc/install).

### 1. Set up the broker

> It is not possible to use the already set up Snowflake broker
> maintained by The Tor Project because as of 2024
> it refuses clients who ask to connect to servers other than
> the ones maintained by The Tor Project.  
> This might change in the future. Keep an eye on
> [this issue](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40166).

1. Download the original Snowflake code.

    ```bash
    git clone https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake.git
    ```

1. _

    ```bash
    cd snowflake/broker
    ```

1. Create a list of available Snowflake servers.
    Replace `wss://my-server-1.my-domain.com:7901` with the URL
    that your Snowflake server (not the broker!) is gonna listen on.
    We're gonna set up the said server in the next steps.

    ```bash
    echo '{"displayName":"my", "webSocketAddress":"wss://my-server-1.my-domain.com:7901", "fingerprint":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}' > bridgeListMy.txt
    ```

    > This step might become unnecessary after
    > [this MR](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/379)
    > is merged.

1. Run the broker.
    Follow
    [the original instructions](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/tree/main/broker?ref_type=heads#running-your-own)
    to set up TLS ecnryption.

    Or you can run it ⚠️ without encryption.
    Again, replace `my-server-1.my-domain.com`
    with the hostname of your Snowflake server,
    and `localhost:4444` with the address that you want the broker to listen on.
    <!-- TODO after
    https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/merge_requests/381
    we're gonna say "host" instead of "hostname" and add port number. -->

    ```bash
    go run . \
        -disable-geoip \
        -disable-tls \
        -addr=localhost:4444 \
        -allowed-relay-pattern='^my-server-1.my-domain.com$' \
        -bridge-list-path=bridgeListMy.txt
    ```

### 2. Set up a proxy

1. Download the original Snowflake code.

    ```bash
    git clone https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake.git
    ```

1. _

    ```bash
    cd snowflake/proxy
    ```

1. Run the proxy.

    Replace `http://localhost:4444` with the URL of your broker,
    and `^my-server-1.my-domain.com$` with the same pattern
    that you set for the broker (i.e. your server's hostname).  
    Omit `keep-local-addresses` in production.

    ```bash
    go run . \
        -broker='http://localhost:4444' \
        -verbose \
        -allowed-relay-hostname-pattern='^my-server-1.my-domain.com$' \
        -allow-non-tls-relay \
        -keep-local-addresses
    ```

### 3. Set up the server

1. Download this project's code.

    ```bash
    git clone https://github.com/WofWca/snowflake-generalized.git
    ```

1. _

    ```bash
    cd snowflake-generalized/server
    ```

1. Run the server.

    Replace `example.com:80` with the desired destination.
    In practice you'd want it to be a
    [SOCKS](https://github.com/serjs/socks5-server)
    / VPN / Tor server running on the same machine as the server.
    Also replace `localhost:7901` with `:7901` if you want the server
    to be publicly reachable.
    ⚠️ Remove `-disable-tls` and add `acme-hostnames=...` to enable encryption.

    ```bash
    go run . \
        -destination-address='example.com:80' \
        -listen-address='localhost:7901' \
        -disable-tls
    ```

### 4. Run the client

1. Download this project's code.

    ```bash
    https://github.com/WofWca/snowflake-generalized.git
    ```

1. _

    ```bash
    cd snowflake-generalized/client
    ```

1. Run the client!

    Again, replace `broker-url` with the URL of the broker
    from a previous step
    and (optionally) `server-id` with the one that you used
    in `bridgeListMy.txt`.  
    Omit `keep-local-addresses` in production.

    ```bash
    go run . \
        -listen-address='localhost:2080' \
        -broker-url='http://localhost:4444' \
        -server-id='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' \
        -keep-local-addresses
    ```

Now open up the browser and go to <http://localhost:2080>.
If you see a dummy 404 page, then it worked!

Now feel free to replace `example.com:80` with a real service of your choosing.

<!-- ### Example setup with a SOCKS proxy

### Example setup with Tor -->

## Similar projects

<!-- TODO should we address why we still work on this project,
despite many competitors existing? Perhaps together with the
"why did you make this project" section.
(spoiler: because Snowflake is the core,
and Tor Project provides long-term support for it). -->

- [Snowflake](https://snowflake.torproject.org/).
    Well, this project is based on Snowflake.
- [Snowstorm](https://snowstorm.net/).
    A WIP project by the original Snowflake author.
    Seems very similar to this project in spirit.
    Claims to be optimized for high speed and low latency.
- [Lantern's Unbounded](https://unbounded.lantern.io/)
([source code](https://github.com/getlantern/unbounded)).
    Also very similar.
    Development started after Snowflake has been released,
    so it hopefully avoids unfortunate design decisions of Snowflake,
    for example it's already using
    [WebTransport](https://developer.mozilla.org/en-US/docs/Web/API/WebTransport),
    as opposed to
    [WebSocket](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
    used by Snowflake. WebTransport should be faster.
- [Ceno Browser](https://censorship.no/).
    Similar because peer-to-peer, but seems to be webpage-oriented,
    i.e. it fetches cached versions from peers using BitTorrent.
    But there is a lot more to it, so better check
    [their manual](https://censorship.no/user-manual/en/concepts/how.html).
- [MassBrowser](https://massbrowser.cs.umass.edu/).
    The difference is that proxies act as exit nodes
    and not just middle nodes like in Snowflake and in this project,
    so it's not as safe to run a proxy.
    And the project seems a bit abandoned as of 2025-01.
- [SpotProxy](https://github.com/spotproxy-project/spotproxy/).
    Doesn't seem super similar,
    but there is a similarity in the fact that proxies are ephemeral,
    and there is explicit support for switching between them.

Please let me know if you know other similar projects!

## Addendum
<!-- This is an implementation of the idea proposed in the
["Snowflake as a generic TCP (UDP?) forwarder (like `ssh -L`)"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40168)
issue. -->

Thanks to The Tor Project for making [the Snowflake library](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/) easy to use!
