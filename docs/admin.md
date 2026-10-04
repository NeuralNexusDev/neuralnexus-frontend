# How the admin pages work

The admin pages, the account page and the bee suggestion review page work the same way. The server sends a nearly empty page first. The browser then asks for the content and htmx swaps it in. Use this page to find where to change something.

## The page and its content

The first response is a shell. It has the heading, the back links, the error banner and a placeholder that says "Loading…". `shell` and `shellFor` in `page.go` serve it. The shell holds no API data, and the server sends it with `Cache-Control: no-store`.

As soon as the page loads, the placeholder asks the server for the real content, and the answer replaces it. Two wrappers handle those requests.

- `pageRoute` wraps handlers that read. It gives the request a 30 second limit and an `apiSession`, which calls nn-api with the visitor's `session` cookie.
- `pageAction` wraps handlers that change something. It reads the form and requires the header `HX-Request: true`, which a form on another site cannot send.

A middleware around the whole router adds a second check. It rejects any request that is not GET, HEAD or OPTIONS unless it has `HX-Request: true`. It also rejects a `Sec-Fetch-Site` of `cross-site` or `same-site`. Paths that match no route, or only the catch-all, keep their normal answer.

`webserver.go` lists the routes.

## When something fails

`failFragment` answers a failure with the status the API returned. It puts the message in the error banner by sending `HX-Retarget: #page-error` and `HX-Reswap: innerHTML`. It can also send extra fragments that restore a row, a field or a status line.

- A 401 means the session ended, so the server sends `HX-Redirect: /login` instead of a message.
- `failEditor` does the same as `failFragment` and also empties the editor's status line.
- A 400, 409 or 422 on a role name, permission node or username also redraws that field with `aria-invalid`, the message and focus.
- A reload that fails after a write succeeded goes through `afterWrite`, which says the change was made.

The server logs failures with a status of 500 or more. The log line has the request ID, the API call and the cause. It never has the cookie or the form.

## What happens while a request runs

The htmx script in `public/js/htmx-glue.js` handles this. It loads before htmx on pages that use htmx.

While a request runs, the element that sent it and its region look busy. The script sets `aria-busy="true"` on both, and a rule in `assets/css/input.css` dims them.

- If someone clicks twice, or clicks another button in the same region, the script drops the second click and shows a message in the banner.
- If someone types or ticks a box while a save runs, the script puts the text and ticks back after the page redraws, with the focus and caret in place. The status line says they are not saved yet.
- If someone changes a checkbox or select in a region where the response redraws every control, the script undoes the change.
- If someone changes the select in the grant form while a request runs, and the response then redraws that form, the script drops the whole form. The value field belongs to the permission it was typed for.

The templates and the script share a few names. The script finds the page by them, so a rename has to change both.

- `data-busy-region` marks an element that runs one request at a time. The value `replaced` means the response redraws every control inside it.
- `data-status` on a region names the status line that shows `Saved` and the unsaved note. Only the user and role editors have one.
- `data-page-load` marks a shell placeholder. While its request runs, the script writes `Loading…` and then `Loaded` into `#page-status`.
- `#page-error` is the banner. Both the script and `failFragment` write to it.

## Where to find things

- `components/ui.templ` has the shared classes and components, such as `pageShell`, `pageHeading`, `formField`, `textInput`, `toggle`, `emptyState` and `busyAttrs`.
- `components/admin.templ` has the admin components, and `components/account.templ` has the account ones.
- `page.go` has what every page shares: the route wrappers, the shells, the failure helpers and the log line.
- The handlers are in `admin*.go`, `account.go` and `bee.go`.
- `api.go` has the API client and `apiError`.
