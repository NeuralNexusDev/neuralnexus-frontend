# Test plan: accessibility

This plan is by topic and not by source file. It covers what only a real render shows about accessibility: computed colours and contrast, the focus ring, the size of a target, what is visible and what takes room, the announcements of the live regions and the state a field is left in after a swap. The markup itself (the roles, the landmarks, `aria-describedby`, `aria-invalid` on a field the server renders as invalid, the title of each page) is pinned by the Go plans and is not repeated here. The tests are in test/js/tests/accessibility.spec.js, and each title starts with its row ID.

Every test signs in through `signIn` with the stub seed named in its precondition. A contrast row has two cases, one for each system colour scheme set with `page.emulateMedia`, and they are a loop of 2 tests. Contrast is the WCAG ratio of the computed colours, with a translucent colour blended over what is behind it. A border or a track is held to 3:1 and text to 4.5:1.

## Contrast

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-01 | form control border | Happy Path | A text field's border is visible against the page | Default seed; `/admin/roles`; a light system, then a dark system | The border colour of `#admin-role-create-name` is at least 3:1 against the page background | P0 | A loop of 2 tests |
| AX-02 | error banner | Happy Path | The banner text is readable on its own tint and its border stands out from the page | Default seed; `/admin/roles`; the text "Something went wrong" put in `#page-error`; a light system, then a dark system | The text is at least 4.5:1 against the banner's background blended over the page, and the banner's border at least 3:1 against the page | P0 | A loop of 2 tests |
| AX-03 | switch | Happy Path | The off track of a switch is visible against the page | `account` `testuser` with `password_auth` false; `/account`; a light system, then a dark system | The track behind `#password-auth-enabled` is at least 3:1 against the page background | P0 | A loop of 2 tests |
| AX-04 | page description | Happy Path | The muted description under a heading is readable | Default seed; `/admin/roles`; a light system, then a dark system | The text colour of the paragraph after the `h1` is at least 4.5:1 against the page background | P1 | A loop of 2 tests. The muted colour is the weakest text colour the pages use |
| AX-05 | field error message | Happy Path | The message under a refused field is readable | `POST /roles` failing once with 409 "A role with this name already exists"; `/admin/roles`; `moderator` submitted; a light system, then a dark system | The text colour of `#admin-role-create-name-error` is at least 4.5:1 against the page background | P1 | A loop of 2 tests. The message sits on the page and not on the banner's tint |

## Focus ring

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-06 | text field and button focus ring | Happy Path | Both show a ring that is visible from the keyboard, none from the mouse | Default seed; `/admin/roles`; a light system, then a dark system; a mouse press on `#admin-role-create-submit`, then Tab and focus on the button and on `#admin-role-create-name` | Before any focus both have no box shadow. The mouse press focuses the button and leaves its box shadow `none`. From the keyboard each shows a box shadow whose outer ring is at least 3:1 against the page background | P0 | A loop of 2 tests. The ring is the outermost ring of the computed box shadow |
| AX-07 | switch | Happy Path | A switch shows a focus ring on its track from the keyboard | `account` `testuser` with `password_auth` true; `/account`; Tab, focus on the switch, Shift+Tab, Tab | The switch is focused and the box shadow of its track goes from `none` to a ring | P0 | The input is visually hidden, so the ring is drawn on the track |

## Target size

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-08 | switch | Happy Path | A switch is large enough to hit | `account` `testuser` with `password_auth` true; `/account` | The track is at least 24 px tall and 24 px wide | P0 | |
| AX-09 | Link and Unlink buttons | Happy Path | Each button is large enough to hit and its negative margin keeps the row layout | `account` `testuser` with `password_auth` true and `myLinks` of a verified Discord link and an unverified Twitch link; `/account`; the buttons `#link-discord-action`, `#link-twitch-action`, `#link-microsoft-action`, `#link-xboxlive-action` and `#link-steam-action` | Each button is at least 24 px wide and 24 px tall and has a computed `marginTop` of `-4px` | P0 | One test over the 5 buttons, so Link and Unlink are both seen |

## Page structure

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-10 | skip link | Happy Path | It is hidden until it has focus and moves focus to the main landmark | Default seed; `/admin`; Tab, then Enter | Before: the link is at most 1 px wide. After Tab: it is focused and more than 50 px wide. After Enter: the URL ends in `#main`, the main landmark is focused and has no outline | P0 | |
| AX-11 | error banner | Edge Case | The empty banner takes no room | Default seed; `/admin/roles` | The box of `#page-error` is 0 px tall and its computed `marginBottom` is `0px` | P0 | |

## Live regions

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-12 | shell live region | Happy Path | A shell announces Loading… and then Loaded through a region that is already on the page | Default seed; `GET /roles` held; `/admin/roles`; a `MutationObserver` on `#page-status` added at `DOMContentLoaded`; the held request then released | While held `#page-status` reads `Loading…`; after the release the list shows and it reads `Loaded`. The observer saw exactly `Loading…`, then `Loaded` | P0 | The region exists at `DOMContentLoaded`, so the first text is announced |
| AX-13 | shell live region | Error Handling | A shell whose content fails with an HTTP error drops the Loading line and says so | `GET /users` failing once with 500 "users are down"; `/admin/users` | `#page-error` is not empty, no `Loading…` text is on the page and `#page-status` reads `Failed to load` | P0 | |
| AX-14 | shell live region | Error Handling | A shell whose content cannot reach the server drops the Loading line and says so | Default seed; the request for `/admin/roles/list` aborted by a route; `/admin/roles` | No `Loading…` text is on the page and `#page-status` reads `Failed to load` | P1 | No response arrives, which is a different path from a response with a status |
| AX-15 | users count line | Happy Path | Search and Load more announce a count in a line that is on the page from the start | `generateUsers` 247; a listener added at `DOMContentLoaded` that records the text of `#admin-users-count`; `/admin/users`; Load more; a search for `user-3`; a search for `zzzzzz` | The line is present and empty at `DOMContentLoaded`. Then it reads `200 users`, `50 more users` after Load more, something else after `user-3`, and `No matches…` after `zzzzzz` | P0 | |
| AX-16 | Saved status | Happy Path | The Saved status goes away when the form is edited again | Default seed; `/admin/users/{bob}`; the username changed to `robert` and saved; a character typed into the field | `#admin-user-status` reads `Saved`, then is empty after the next input | P1 | |

## Fields

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AX-17 | refused field | Error Handling | A refused role name is described by the message and keeps focus and its value | `POST /roles` failing once with 409 "A role with this name already exists"; `/admin/roles`; `moderator` submitted | The banner and `#admin-role-create-name-error` both read the message, the field's accessible description contains it, and the field is focused with `moderator` still in it | P0 | The accessible description is what a screen reader reads, so it proves that the reference resolves |
| AX-18 | refused field | Edge Case | Editing a refused field clears its flag and its message | As AX-17; a character typed into the field | The field has `aria-invalid` `true` before and not after, `#admin-role-create-name-error` is gone, and the accessible description no longer contains the message but still contains the help text, "Lower-case letters, digits and underscores…" | P0 | The script removes the flag on input |
