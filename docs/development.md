# Development

If you want to contribute to traQ, then follow this section.

### Requirements

- Go 1.19
- git
- bash
- make
- Docker
- docker-compose

### Setup Local Server with Docker

#### First Up (or entirely rebuild)
`make up`

Now you can access to
+ `http://localhost:3000` for traQ
    + admin user id: `traq`
    + admin user password: `traq`
+ `http://localhost:3001` for Adminer
+ `http://localhost:6060` for traQ pprof web interface
+ `3002/tcp` for traQ MariaDB
    + username: `root`
    + password: `password`
    + database: `traq`
+ `http://localhost:3003/healthz` for the BOT_MAI health check
+ `http://localhost:3004/healthz` for the BOT_AI health check

`make up` also builds and starts `bot/BOT_MAI` and `bot/BOT_AI`. In the development Compose environment, both bot accounts, tokens, subscriptions, and activation are configured automatically; an external tunnel is not required. See [`bot/BOT_MAI/README.md`](../bot/BOT_MAI/README.md) and [`bot/BOT_AI/README.md`](../bot/BOT_AI/README.md) for details.

The `frontend` service builds the local sibling repository at `../1m26_02-client` with `Dockerfile.local`, rather than downloading a released frontend. Keep `1m26_02-server` and `1m26_02-client` under the same parent directory. The local Caddy image serves the built client and proxies `/api/*` and `/.well-known/*` to the backend, so `make up` starts the server, matching client, q_bot, MariaDB, and supporting services together.

The frontend container is disabled for local development. Start the client repository's Vite development server separately and open `http://localhost:8080`.

```shell
npm run dev
```

For a release, re-enable the `frontend` service in `compose.yaml` and disable the backend's `3000:3000` host port mapping to avoid a port conflict.

#### Rebuild traQ
`make up`

#### Destroy Containers
`make down`

#### Remove dev data
1. `make down`
2. Remove respective directory in `./dev/data` (e.g. to remove all `rm -r ./dev/data/*`)
3. `make up`

#### Build executable file
`make traQ`

#### Download and Install go mod dependencies
`make init`
> `github.com/google/wire/cmd/wire` and `github.com/golang/mock/mockgen` will be installed.

#### Rerun automated code generation (wire, gomock)
`make gogen`

#### Testing
1. Setup test DB container by `make up-test-db`
2. `make test`
3. (Remove test DB container by `make rm-test-db`)

#### Code Lint
`make lint` (or individually `make golangci-lint`, `make swagger-lint`)

Powered by:
+ [golangci-lint](https://github.com/golangci/golangci-lint) for go codes (pre-installation required)
+ [spectral](https://github.com/stoplightio/spectral) for swagger specs

#### Generate and Lint DB Schema Docs
If your changelist alters the database schema, you should regenerate db docs.

1. Write new schema descriptions in `.tbls.yml`.
2. Make sure the Test DB Container is running (run `make up-test-db`).
3. `make db-gen-docs`

Powered by:
+ [tbls](https://github.com/k1LoW/tbls) for generating schema docs
