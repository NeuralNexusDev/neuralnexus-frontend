# The admin pages

The admin, account and bee review pages share one pattern. The server renders a static shell, and htmx loads the content from a fragment route.

## Shell and fragments

A shell route (`shell`, `shellFor`) serves a page with the heading, back links, the error banner `#page-error`, a status region `#page-status` and a placeholder `<div data-page-load hx-trigger="load">`. It holds no API data, and the server sends it with `Cache-Control: no-store`. The placeholder fetches its fragment and replaces itself with it.

- `pageRoute` wraps a read fragment. It gives the request a 30 second context and an `apiSession`, which calls nn-api with the visitor's `session` cookie.
- `pageAction` wraps a change. It requires `HX-Request: true` and parses the form.
- `requireHTMXForWrites` wraps the whole mux. It rejects a request that is not GET, HEAD or OPTIONS unless it has `HX-Request: true`, and rejects a `Sec-Fetch-Site` of `cross-site` or `same-site`. Unmatched paths and the catch-all route keep their normal answer.

## Routes

| Route | Returns |
|---|---|
| `GET /admin/cards` | The cards for the admin pages the account may use |
| `GET /admin/users/list`, `/admin/users/rows` | The search box and the first page of users, then later pages and the count line |
| `GET /admin/users/{id}/editor`, `POST /admin/users/{id}` | The user editor, then the form, header and permissions after a save |
| `GET /admin/roles/list`, `POST /admin/roles` | The role list and create form, then a redirect to the new role |
| `GET /admin/roles/{id}/editor`, `POST`, `DELETE /admin/roles/{id}` | The role editor, then the form after a rename, then a redirect after a delete |
| `GET /admin/roles/{id}/grant-value` | The value input for the chosen permission |
| `POST`, `DELETE /admin/roles/{id}/permissions[/{permission}]` | The granted list and grant form after a grant, value change or removal |
| `GET /admin/permissions/list`, `POST /admin/permissions`, `DELETE /admin/permissions/{id}` | The permission list and create form, then the new list |
| `GET /account/content`, `POST /account/settings`, `POST`, `DELETE /account/links/{platform}` | The account page, then the redrawn setting or row |
| `GET /project/bee-name-generator/admin/suggestions`, `POST` | The pending suggestions, then the list after a review |

## Errors

`failFragment` answers a failure with the API's status. It sets `HX-Retarget: #page-error` and `HX-Reswap: innerHTML` so the message appears in the banner, and it can add out-of-band fragments that restore a row, a field or a status line. A 401 sends `HX-Redirect: /login` instead. `failEditor` also empties the editor's status line. A 400, 409 or 422 on a role name, permission node or username also redraws that field with `aria-invalid`, the message and focus. A reload that fails after a write goes through `afterWrite`, which says the change was made. The server logs a failure with a status of 500 or more with the request ID, the API call and the cause. It never logs the cookie or the form.

## The glue

`public/js/htmx-glue.js` loads before htmx on pages that use it. It depends on these markers.

- `data-busy-region` marks an element that runs one request at a time. The value `replaced` means the response redraws every control inside it.
- `data-status` on a region names the status line that shows `Saved` and the unsaved note. Only the user and role editors have one.
- `data-page-load` marks a shell placeholder. Its request writes `Loading…` and `Loaded` into `#page-status`.
- `#page-error` is the banner that the glue and `failFragment` write to.

While a request runs, the glue sets `aria-busy="true"` on the requesting element and its region, and the base CSS rule in `assets/css/input.css` dims them. The glue drops a submit or button click in a busy region and reports it in the banner. It undoes a checkbox or select change in a `replaced` region. In the other regions it puts back the text and ticks typed during the request after the redraw, with the focus and caret, and the status line says they are not saved yet. It drops a grant form as a whole when its select changed.

## Where things live

The shared classes and components (`pageShell`, `pageHeading`, `formField`, `textInput`, `toggle`, `emptyState`, `busyAttrs`) are in `components/ui.templ`. The admin components are in `components/admin.templ`, the account ones in `components/account.templ`, and the handlers in `admin*.go`, `account.go` and `bee.go`. `api.go` holds the API client and `apiError`.
