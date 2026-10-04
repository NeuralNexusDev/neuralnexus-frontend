# How the pages load and change

The admin pages, the account page and the bee suggestion review page work the same way. The server sends a nearly empty page first. The browser then asks for the content and htmx swaps it in.

## The page and its content

The first response is a shell. It has the heading, the back links, the error banner and a placeholder that says "Loading…". `shell` and `shellFor` in `page.go` serve it. The shell holds no API data, and the server sends it with `Cache-Control: no-store`.

As soon as the page loads, the placeholder asks the server for the real content, and the answer replaces it. Two wrappers handle those requests.

- `pageRoute` wraps handlers that read. It gives the request a 30 second limit and an `apiSession`, which calls nn-api with the visitor's `session` cookie.
- `pageAction` wraps handlers that change something. It reads the form and requires the header `HX-Request: true`, which a form on another site cannot send. The read-only route that loads the value field of the grant form uses it too, so that it also requires the header.

A middleware around the whole router adds a second check. It rejects any request that is not GET, HEAD or OPTIONS unless it has `HX-Request: true`. It also rejects a `Sec-Fetch-Site` of `cross-site` or `same-site`. Paths that match no route, or only the catch-all, keep their normal answer.

`webserver.go` lists the routes.

## When something fails

`failFragment` answers a failure with the status the API returned. It puts the message in the error banner by sending `HX-Retarget: #page-error` and `HX-Reswap: innerHTML`. It can also send extra fragments that restore a row, a field or a status line.

- A 401 means the session ended, so the server sends `HX-Redirect: /login` instead of a message.
- `failEditor` does the same as `failFragment` and also empties the editor's status line.
- A 400, 409 or 422 on a role name, permission node or username also redraws that field with `aria-invalid`, the message and focus.
- A reload that fails after a write succeeded goes through `afterWrite`, which says the change was made.

The server logs failures with a status of 500 or more. The log line has the request ID, the route pattern of the API call, such as `PUT /roles/{id}`, and the cause. It does not have the path, so an ID or a name that someone typed stays out of the log. It never has the cookie, the form or anything else a person typed.

## What happens while a request runs

The htmx script in `public/js/htmx-glue.js` tracks the requests that run and tells the person what happened. It loads before htmx on pages that use htmx.

While a request runs, the element that sent it and its region are marked busy. The script sets `aria-busy="true"` on both, and rules in `assets/css/input.css` dim them and stop the mouse from reaching the region.

- htmx runs one request at a time in a region. `hx-sync` with `this:drop` makes it drop a second request while the first runs. The script only reports the drop. A mouse click never reaches a busy region, so the report shows for keyboard and submit actions, such as pressing Enter in a field.
- A dropped action always shows "The last change is still being saved. Try again in a moment." in the banner. When the action repeats the first one, which means the same button over the same field values, that is all. When the action carries a changed field or a different button, the banner also says "The change made while saving was not sent. Make it again." once the save ends.
- If someone types or ticks a box while a save runs, the script puts the text and ticks back after the page redraws, with the focus and caret in place. The status line says they are not saved yet.
- If someone changes a checkbox or select in a region where the response redraws every control, the script undoes the change.
- If someone changes the select in the grant form while a request runs, and the response then redraws that form, the script does not put the typed values back into it. Each value field belongs to one permission, so a value typed for the old choice would sit under the new one. The banner says "The change made while saving was not sent. Make it again."
- If a newer request replaces one that is still running, such as a new search or a new selection, the script drops the old request, and only the newest answer shows. A replaced request does not log an error in the console.
- If a request times out or the network fails, the banner says "The server could not be reached. Try again in a moment."
- If the content of a page fails to load, the status line says "Failed to load", and the "Loading…" line goes away. The error banner shows why. A load that works ends with "Loaded".
- If someone edits a field in the user or role editor, the script clears the "Saved" status line. If the field has an error, the script also clears it, which means its `aria-invalid` and its message.

The templates and the script use a few names to find elements. A rename has to change both.

- `data-busy-region` marks an element that runs one request at a time. The value `replaced` means the response redraws every control inside it.
- `data-status` on a region names the status line that shows `Saved` and the unsaved note. Only the user and role editors have one.
- `data-page-load` marks a shell placeholder. While its request runs, the script writes `Loading…` into `#page-status`, then `Loaded` or `Failed to load`.
- `#page-error` is the banner. Both the script and `failFragment` write to it.

## Where to find things

- `components/ui.templ` has the shared classes and components, such as `pageShell`, `pageHeading`, `formField`, `textInput`, `toggle`, `emptyState` and `busyAttrs`.
- `components/admin.templ` has the admin components, and `components/account.templ` has the account ones.
- `page.go` has what every page shares: the route wrappers, the shells, the failure helpers and the log line.
- The handlers are in `admin*.go`, `account.go` and `bee.go`.
- `api.go` has the API client and `apiError`.
