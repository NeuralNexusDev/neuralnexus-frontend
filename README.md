# neuralnexus-frontend

The frontend for neuralnexus.dev. A Go server renders pages with templ and calls nn-api with the visitor's session cookie.

## Stack

- Go net/http with the 1.22 route patterns
- templ for pages and fragments
- htmx 4, loaded from the S3 CDN with a subresource integrity hash
- Tailwind CSS v4
- Playwright for browser tests, against a stub API

## Run it

The server needs `NN_API_URL` and `NN_SITE_URL`, both http or https URLs. These are optional.

- `ADDRESS` is the listen address and defaults to `0.0.0.0:8090`. `USE_UDS=true` listens on a Unix socket instead.
- `JWT_SECRET` verifies the session token.
- `DISCORD_CLIENT_ID`, `TWITCH_CLIENT_ID`, `MICROSOFT_CLIENT_ID` and their `*_REDIRECT_URI` variables build the sign-in links.
- `STEAM_OPENID_LOGIN_URL` overrides Steam's OpenID endpoint.

`make dev` sets defaults for these, then runs the Tailwind watcher, the templ watcher and the server with air. `make generate` builds the CSS and the templates once.

## Test it

- `make test-go` generates the templates and runs the Go tests against a fake API.
- `make test-race` runs the same tests with the race detector.
- `make fmt-check`, `make vet` and `make templ-check` run gofmt, go vet and `templ fmt -fail`.
- `make test` starts the stub API, the server and Playwright in Docker, then tears them down. Without Docker, `cd test/integration && npm ci && npx playwright test` starts the server and the stub itself.

The stub API lives in `test/integration`. `stub-api.mjs` answers the mc-status routes and passes `/api/v1` to `stub-state.mjs`, which serves users, roles, permissions, the account and the bee suggestions from state kept per session. A test seeds that state with `signIn(page, state)`, which also returns `writes()` to read what the app changed. The seed can include failures that answer with a status, delays for the htmx timeout specs, and gates. `const held = await gate('PATCH /roles/1')` holds that request, `await held.arrived()` waits until it is in flight, and `await held.release()` lets it finish, optionally with a status and detail.

CI runs gofmt, `templ fmt -fail`, go vet, `go test`, `go test -race` and the Playwright suite, and uploads the Playwright report and traces when a step fails. The admin pages are described in `docs/admin.md`.
