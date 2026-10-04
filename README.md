# neuralnexus-frontend

This is the frontend for neuralnexus.dev. A Go server renders the pages with templ and gets its data from nn-api, using the visitor's session cookie.

## What it uses

- Go net/http with the 1.22 route patterns
- templ for pages and fragments
- htmx 4, loaded from the S3 CDN with a subresource integrity hash
- Tailwind CSS v4
- Playwright for browser tests, against a stub API

## Run it

Set `NN_API_URL` and `NN_SITE_URL` before you start the server. Both must be http or https URLs. You can also set these variables.

- `ADDRESS` is the listen address and defaults to `0.0.0.0:8090`. `USE_UDS=true` listens on a Unix socket instead.
- `JWT_SECRET` verifies the session token.
- `DISCORD_CLIENT_ID`, `TWITCH_CLIENT_ID`, `MICROSOFT_CLIENT_ID` and their `*_REDIRECT_URI` variables build the sign-in links.
- `STEAM_OPENID_LOGIN_URL` overrides Steam's OpenID endpoint.

`make dev` sets default values for the API and site URLs and for the Discord and Twitch settings. It then runs the Tailwind watcher, the templ watcher and the server with air. `make generate` builds the CSS and the templates once.

## Test it

- `make test-go` generates the templates and runs the Go tests against a fake API.
- `make test-race` runs the same tests with the race detector.
- `make fmt-check`, `make vet` and `make templ-check` run gofmt, go vet and `templ fmt -fail`.
- `make test` starts the stub API, the server and Playwright in Docker, then shuts them down. Without Docker, run `cd test/integration && npm ci && npx playwright install chromium && npx playwright test`. That command starts the server and the stub itself, on port 8099 for the site and port 8098 for the stub API. Outside CI it reuses anything that already answers on those ports, so stop an old server or another program that holds them first.

The stub API is in `test/integration`. `stub-api.mjs` answers the mc-status routes and passes `/api/v1` to `stub-state.mjs`. That file serves users, roles, permissions, the account and the bee suggestions from state it keeps for each session.

A test seeds that state with `signIn(page, state)`. The same call returns `writes()`, which lists what the app changed. The seed can include failures that answer with a status, delays for the htmx timeout specs, and gates.

A gate holds one request until the test lets it finish.

- `const held = await gate('PATCH /roles/1')` registers the gate.
- `await held.arrived()` waits until the request is in flight.
- `await held.release()` lets it finish. You can pass a status and detail to make it fail.

CI runs gofmt, `templ fmt -fail`, go vet, `go test`, `go test -race` and the Playwright suite. When a step fails, it uploads the Playwright report and traces. `docs/admin.md` explains how the pages load and change.
