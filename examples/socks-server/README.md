# SOCKS server example

FYI there is a publicly accessible service that implements this setup.
See the ["Try it" section in root README](../../README.md#try-it).

This example sets up a SOCKS server that
is accessible through a Snowflake tunnel.

⚠️ Note that running a public SOCKS server
could subject you to legal issues.  
Setting a password on the SOCKS proxy
in order to make it private won't really work,
because proxies can intercept the password
since it's transferred in plain text.  
If you want to set up a personal VPN,
there are better solutions, such as [Amnezia](https://amnezia.org/en/self-hosted).

1. Clone this repo and `cd` into its root.
2. [Install Docker](https://docs.docker.com/engine/install/).
3. Obtain a domain name.
    You can get one easily and for free on <https://www.duckdns.org/>.
4. Point the domain name to your VPS's IP.
5. Create an `.env` file as in <./example.env>.
    Adjust `ACME_HOSTNAMES`.
6. Execute

    ```bash
    docker compose \
        --file examples/socks-server/docker-compose.yml \
        --env-file=examples/socks-server/.env \
        up --build --detach
    ```

For purely local testing deployment you don't need a domain name.
You can set `DISABLE_TLS=true` in the `.env` file, and omit other variables.

The server setup is done! Now, to use the server:

1. Launch the client on another machine as such:

    ```bash
    go run ./client \
        --broker-url=https://<APPROPRIATE_BROKER> \
        --server-url=wss://<YOUR_DOMAIN>:7901 \
        --listen-address=localhost:2080
    ```

    (The broker setup is described in README at the root of this project).
2. Go to your browser's proxy settings and set it to use a SOCKS server
    at address = `localhost`, port = `2080`.

Your browser is now connected to the SOCKS server
throught a Snowflake tunnel!

You can adjust SOCKS server config
(such as allowed destinations, username/password)
by creating a `.socks-server.env` file.
Supported variables are listed [here](https://github.com/serjs/socks5-server?tab=readme-ov-file#list-of-supported-config-parameters).  
⚠️ Note that the SOCKS username / password can be intercepted
by Snowflake proxies:
the client \<-\> server connection
is not end-to-end encrypted.

If you want the SOCKS server to only allow your own website as a destination,
set `ALLOWED_DEST_FQDN=my-website.com` in `.socks-server.env`.  
Then you can direct your users to utilize tools such as
[SmartProxy](https://github.com/salarcode/SmartProxy)
to use their Snowflake client as a SOCKS server only for your own website.  
Or, if you maintain an app that only connects to your website,
integrate the Snowflake client in your app and make the rest of the app
use it as a SOCKS server.

To stop the server:

```bash
docker compose --file examples/socks-server/docker-compose.yml stop
```
