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

`make dev` sets default values for the site URL and for the Discord and Twitch settings, and defaults `NN_API_URL` to `http://0.0.0.0:8080`, where a local nn-api listens. Set `NN_API_URL` yourself to use another API. It then runs the Tailwind watcher, the templ watcher and the server with air, and Ctrl-C stops all three.

## Test it

The Makefile has these targets.

- `make update` updates the templ, templui, air and gotailwind tools to their latest versions.
- `make generate` builds the CSS and the templates.
- `make vet` runs `make generate`, then the gofmt check, `templ fmt -fail` and `go vet`.
- `make dev` runs the site locally, as described above.
- `make build` runs `make vet`, then builds the server to `build/webserver`. The Dockerfile runs it, so every image build checks formatting and runs `go vet` first. It does not run the tests.
- `make test` runs `make test-go` and then `make test-js`, and stops at the first failure.
- `make test-go` runs `make generate`, then `go test -race ./...` against a fake API. The race detector needs a C compiler.
- `make test-js` starts the stub API, the server and Playwright in Docker, then shuts them down and passes on the exit status of Playwright.

Without Docker, run `cd test/js && npm ci && npx playwright install chromium && npx playwright test`. That command starts the server and the stub itself, on port 8099 for the site and port 8098 for the stub API. Outside CI it reuses anything that already answers on those ports, so stop an old server or another program that holds them first.

The stub API is in `test/js`. `stub-api.mjs` answers the mc-status routes and passes `/api/v1` to `stub-state.mjs`. That file serves users, roles, permissions, the account and the bee suggestions from state it keeps for each session.

A test seeds that state with `signIn(page, state)`. The same call returns `writes()`, which lists what the app changed. The seed can include failures that answer with a status, delays for the htmx timeout specs, and gates.

A gate holds one request until the test lets it finish.

- `const held = await gate('PATCH /roles/1')` registers the gate.
- `await held.arrived()` waits until the request is in flight.
- `await held.release()` lets it finish. You can pass a status and detail to make it fail.

CI runs `make vet` and `make test`. When a step fails, it uploads the Playwright report and traces.
