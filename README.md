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

<!-- ## Can I run a proxy to help others circumvent censorship? -->

## Is it production-ready?

Not yet.

### It's not actually faster

<!-- FYI this section is linked to in ./examples/wireguard/README.md -->

As much as I hate to admit it, this project is, well, _not actually faster_
than regular Snowflake with Tor.
Yep. As of 2025-01, after a bunch of tests, mainly with a SOCKS server
(see [./examples/socks-server](./examples/socks-server)),
I have not been able to achieve
a sustained download speeds greater than 8 Mbit / s.  
Latency, however _is_ actually lower due fewer hops,
but it's still not suitable for, say, gaming.

So, even with Tor, Snowflake itself seems to be the bottleneck.

But all is not lost!
The Tor Project team is working on performance improvements.
One such improvement is
["Use unreliable and unordered WebRTC data channels"](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/issues/40352),
which gives highest hopes.  

For reference, here are some other performance-related issues,
which also include performance measurements:

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

## Why did you make the project?

My dream is: Snowflake clients can access _any_ service they want
with the help of Snowflake proxies,
without having to route the traffic through the (not the fastest) Tor network,
and the Snowflake proxies don't have to worry about being abused,
and that all VPN providers have a Snowflake server running,
ready to accept clients for whom their service is blocked.

Snowflake is a fascinating concept and I think it has a lot of potential.

## Usage

Also see <./examples>

***

This guide will walk you through setting up _an entire_ Snowflake network,
with all 4 of its components.  
However, as a regular user, or a service provider,
you'll only need to set up one:
users will need to set up the client,
and service providers will need to set up the server.
You're supposed to be using the broker and the proxies that other people
already set up publicly.

Let's make a setup that will work in the same way as

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
