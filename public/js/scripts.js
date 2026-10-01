/**
 * @description Reads the backend's base URL from the hidden element
 * WrapContents renders on every page - lets tests point the frontend at a
 * different backend without editing this file.
 * @returns {string}
 */
function apiBaseUrl() {
    return document.getElementById('api-base-url').innerText;
}

/**
 * @description Reads the RFC 9457 problem the API embeds in a "problem"
 * query param when an OAuth/OpenID redirect fails (auth.go's
 * redirectWithError) - base64 (URL-safe) encoded, same alphabet as the
 * "state" param but padded, since it's produced by Go's base64.URLEncoding
 * rather than this file's own hand-rolled encodeState(). Shows the
 * problem's detail in the page's #auth-error banner and strips the param
 * from the URL so a refresh or share doesn't repeat it.
 */
function showAuthErrorFromQuery() {
    const params = new URLSearchParams(window.location.search);
    const problemB64 = params.get('problem');
    if (!problemB64) {
        return;
    }

    params.delete('problem');
    const cleanQuery = params.toString();
    const cleanUrl = window.location.pathname + (cleanQuery ? `?${cleanQuery}` : '') + window.location.hash;
    history.replaceState(null, '', cleanUrl);

    try {
        const json = atob(problemB64.replace(/-/g, '+').replace(/_/g, '/'));
        const problem = JSON.parse(json);
        const banner = document.getElementById('auth-error');
        if (banner) {
            banner.textContent = problem.detail || 'Something went wrong. Please try again.';
            banner.hidden = false;
        }
    } catch (error) {
        console.error('Error:', error);
    }
}

/**
 * @description Reads Steam's OpenID login endpoint from the hidden element
 * WrapContents renders on every page - defaults to the real Steam endpoint,
 * overridable so tests can point it at a local stand-in instead of routing
 * around a hardcoded steamcommunity.com literal.
 * @returns {string}
 */
function steamOpenIdLoginUrl() {
    return document.getElementById('steam-openid-login-url').innerText;
}

/**
 * @description Works out the cookie Domain that lets the site and the API
 * share a cookie: the labels their hostnames have in common, when that's
 * at least two (a bare TLD can't be a cookie domain). Returns '' when the
 * hosts are identical, either is an IP address, or they share too little -
 * a host-only cookie is already enough (or the browser would reject the
 * attribute outright).
 * @param {string} siteHost - Hostname the page is served from
 * @param {string} apiHost - Hostname of the API
 * @returns {string} - Domain attribute value such as ".example.com", or ''
 */
function sharedCookieDomain(siteHost, apiHost) {
    const isIp = (host) => /^[\d.]+$/.test(host) || host.includes(':');
    if (siteHost === apiHost || isIp(siteHost) || isIp(apiHost)) {
        return '';
    }
    const siteLabels = siteHost.split('.').reverse();
    const apiLabels = apiHost.split('.').reverse();
    const shared = [];
    while (shared.length < siteLabels.length && siteLabels[shared.length] === apiLabels[shared.length]) {
        shared.push(siteLabels[shared.length]);
    }
    return shared.length >= 2 ? `.${shared.reverse().join('.')}` : '';
}

/**
 * @description Generates a fresh nonce for the OAuth/OpenID flow and sets
 * it as a short-lived cookie the API checks on the callback, returning the
 * nonce. Called at the moment the user clicks a login/link button (not on
 * page load) so its 5-minute TTL covers the provider round-trip rather than
 * however long the user sat on the page first. When the site and API are on
 * different subdomains the cookie is scoped to their shared parent domain so
 * the API receives it; otherwise (e.g. localhost) it stays host-only.
 * SameSite=None/Secure only apply over https.
 * @returns {string} - The generated nonce
 */
function createNonce() {
    const nonce = Math.random().toString(36).substring(2, 15);

    const domain = sharedCookieDomain(location.hostname, new URL(apiBaseUrl()).hostname);
    const domainAttr = domain ? `; domain=${domain}` : '';
    const secureAttr = location.protocol === 'https:' ? '; Secure' : '';
    const sameSite = secureAttr ? 'None' : 'Lax';
    const expires = new Date(Date.now() + 5 * 60 * 1000).toUTCString();
    document.cookie = `nonce=${nonce}; expires=${expires}; path=/${domainAttr}; SameSite=${sameSite}${secureAttr}`;

    return nonce;
}

/**
 * @description Toggles the header's account section (username + settings
 * gear) and Login/Logout button based on whether the session cookie is
 * still valid, checked via /users/me. Runs on every page load since the
 * session cookie is HttpOnly and can't be read from JS.
 */
function checkHeaderAuthState() {
    fetch(`${apiBaseUrl()}/api/v1/users/me`, {
        credentials: 'include'
    })
        .then((res) => {
            const loggedIn = res.ok;
            const loginBtn = document.getElementById('header-login-btn');
            const logoutBtn = document.getElementById('header-logout-btn');
            const accountSection = document.getElementById('header-account');
            if (loginBtn) loginBtn.hidden = loggedIn;
            if (logoutBtn) logoutBtn.hidden = !loggedIn;
            if (accountSection) accountSection.hidden = !loggedIn;
            if (!loggedIn) return;

            return res.json().then((account) => {
                const usernameEl = document.getElementById('header-account-username');
                if (usernameEl && account) usernameEl.innerText = account.username;
            });
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}

function logout() {
    fetch(`${apiBaseUrl()}/api/v1/auth/logout`, {
        method: 'POST',
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 204) {
                window.location.href = '/'
            } else {
                console.error('Logout failed');
            }
        })
        .catch((error) => {
            console.error('Error:', error)
        })
}

function submitLoginForm() {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    const data = Object.fromEntries(formData.entries());
    const json = {
        password: data.password
    }
    const username = data.username
    if (username.includes('@')) {
        json.email = username
    } else {
        json.username = username
    }
    fetch(`${apiBaseUrl()}/api/v1/auth/login`, {
        method: 'POST',
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(json)
    })
        .then((res) => {
            if (res.ok) {
                window.location.href = '/';
                return;
            }
            return res.json().then((problem) => {
                throw new Error(problem.detail || 'Failed to log in');
            });
        })
        .catch((error) => {
            alert(error.message);
        });
}

/**
 * @description an OAuthState object
 * @typedef {Object} OAuthState
 * @property {string} platform - The platform to redirect to
 * @property {string} nonce - The nonce to use for the OAuth flow
 * @property {string} redirect_uri - The redirect URI to use for the OAuth flow
 * @property {string} mode - The mode describing how to handle the OAuth interaction
 */

/**
 * @description This function is used to encode the state object into a string.
 * Uses base64url, not plain base64, since the API decodes it with Go's
 * base64.URLEncoding.
 * @param state {OAuthState} - The state object to encode
 * @returns {string} - The encoded state object
 */
function encodeState(state) {
    return btoa(JSON.stringify(state)).replace(/\+/g, '-').replace(/\//g, '_');
}

/**
 * @description Builds Steam's OpenID 2.0 login request URL. Steam has no
 * OAuth app/client ID to pre-render a base URL from, so unlike the other
 * providers this is built entirely client-side, and state travels inside
 * openid.return_to instead of as a query param appended after the fact.
 * @param state {OAuthState} - The state object to round-trip through Steam
 * @returns {string}
 */
function buildSteamOpenIDURL(state) {
    const returnTo = `${apiBaseUrl()}/api/openid?state=${encodeState(state)}`;
    const params = new URLSearchParams({
        'openid.ns': 'http://specs.openid.net/auth/2.0',
        'openid.mode': 'checkid_setup',
        'openid.return_to': returnTo,
        'openid.realm': `${apiBaseUrl()}/`,
        'openid.identity': 'http://specs.openid.net/auth/2.0/identifier_select',
        'openid.claimed_id': 'http://specs.openid.net/auth/2.0/identifier_select'
    });
    return `${steamOpenIdLoginUrl()}?${params.toString()}`;
}

/**
 * @description Starts an OAuth/OpenID login for platform: mints a fresh
 * nonce right now via createNonce() rather than on page load, then
 * navigates to the provider with the resulting state appended. baseUrl is
 * the pre-rendered authorize URL for OAuth providers, or null for Steam,
 * which has none and builds its whole URL via buildSteamOpenIDURL.
 * @param platform {string}
 * @param baseUrl {string|null}
 */
function startOAuthLogin(platform, baseUrl) {
    let redirect = window.location.href;
    if (redirect.endsWith('/login')) {
        redirect = redirect.substring(0, redirect.length - 6);
    } else if (redirect.endsWith('/register')) {
        redirect = redirect.substring(0, redirect.length - 9);
    }

    const state = { platform: platform, nonce: createNonce(), redirect_uri: redirect, mode: 'login' };
    window.location.href = platform === 'steam' ? buildSteamOpenIDURL(state) : baseUrl + '&state=' + encodeState(state);
}

/**
 * @description Loads the profile into the account page via /me, since the
 * session cookie resolves identity server-side - no user ID needed here.
 */
function loadAccountProfile() {
    fetch(`${apiBaseUrl()}/api/v1/users/me`, {
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (!res.ok) {
                throw new Error('Failed to load account: ' + res.status);
            }
            return res.json();
        })
        .then((account) => {
            if (account) {
                document.getElementById('account-username').innerText = account.username;
            }
        })
        .catch((error) => {
            console.error('Error:', error);
        });

    loadAccountSettings();
    loadLinkedAccounts();
}

/**
 * @description Guards against a stale response repainting the password
 * toggle after a newer request has since been made.
 */
let passwordAuthSeq = 0;

/**
 * @description Loads the caller's account settings and reflects
 * password_auth onto the toggle.
 */
function loadAccountSettings() {
    fetch(`${apiBaseUrl()}/api/v1/users/me/settings`, {
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (!res.ok) {
                throw new Error('Failed to load account settings: ' + res.status);
            }
            return res.json();
        })
        .then((settings) => {
            if (!settings) {
                return;
            }
            const checkbox = document.getElementById('password-auth-enabled');
            if (checkbox) {
                checkbox.checked = settings.password_auth;
                checkbox.disabled = false;
            }
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}

/**
 * @description Toggles whether the account's password can be used to log
 * in. Reverts the checkbox if the request fails, unless a newer request has
 * since been made.
 * @param enabled {boolean}
 */
function setPasswordAuthEnabled(enabled) {
    const seq = ++passwordAuthSeq;

    fetch(`${apiBaseUrl()}/api/v1/users/me/settings`, {
        method: 'PATCH',
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({password_auth: enabled})
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (res.status === 204) {
                return;
            }
            return res.json().then((problem) => {
                throw new Error(problem.detail || 'Failed to update account settings');
            });
        })
        .catch((error) => {
            if (passwordAuthSeq === seq) {
                document.getElementById('password-auth-enabled').checked = !enabled;
                alert(error.message);
            }
        });
}

/**
 * @description The platforms shown as rows on the account settings page.
 */
const LINK_PLATFORMS = ['discord', 'twitch', 'microsoft', 'xboxlive', 'steam'];

/**
 * @description Guards against an out-of-order response repainting the
 * rows with stale data.
 */
let loadLinkedAccountsSeq = 0;

/**
 * @description Fetches the caller's linked accounts and updates each
 * platform row's verified/login-enabled/unlink state, redirecting to
 * /login if the session is missing or expired.
 */
function loadLinkedAccounts() {
    const seq = ++loadLinkedAccountsSeq;

    fetch(`${apiBaseUrl()}/api/v1/users/me/links`, {
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (!res.ok) {
                throw new Error('Failed to load linked accounts: ' + res.status);
            }
            return res.json();
        })
        .then((links) => {
            if (!links || seq !== loadLinkedAccountsSeq) {
                return;
            }
            const byPlatform = {};
            links.forEach((link) => {
                byPlatform[link.platform] = link;
            });
            LINK_PLATFORMS.forEach((platform) => {
                updateLinkRow(platform, byPlatform[platform] || null);
            });
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}

/**
 * @description Reflects one platform's linked-account state onto its row.
 * @param platform {string}
 * @param link {?{platform_username: string, verified: boolean, login_enabled: boolean}}
 */
function updateLinkRow(platform, link) {
    const title = document.getElementById(`link-${platform}-title`);
    const subtitle = document.getElementById(`link-${platform}-subtitle`);
    const verifiedIcon = document.getElementById(`link-${platform}-verified-icon`);
    const status = document.getElementById(`link-${platform}-status`);
    const loginCheckbox = document.getElementById(`link-${platform}-login-enabled`);
    const action = document.getElementById(`link-${platform}-action`);
    if (!title || !subtitle || !verifiedIcon || !status || !loginCheckbox || !action) {
        return;
    }
    const displayName = title.dataset.name;

    if (link) {
        title.textContent = link.platform_username || displayName;
        subtitle.textContent = displayName;
        subtitle.classList.remove('hidden');
        verifiedIcon.classList.toggle('hidden', !link.verified);
        status.textContent = link.verified ? '' : 'Unverified';
        loginCheckbox.checked = link.login_enabled;
        loginCheckbox.disabled = !link.verified;
        action.dataset.linked = 'true';
        action.textContent = 'Unlink';
    } else {
        title.textContent = displayName;
        subtitle.classList.add('hidden');
        verifiedIcon.classList.add('hidden');
        status.textContent = '';
        loginCheckbox.checked = false;
        loginCheckbox.disabled = true;
        action.dataset.linked = 'false';
        action.textContent = 'Link';
    }
}

/**
 * @description The hidden element holding each platform's base OAuth URL.
 */
const LINK_OAUTH_BASE_IDS = {
    discord: 'link-discord-oauth-base',
    twitch: 'link-twitch-oauth-base',
    microsoft: 'link-microsoft-oauth-base',
    xboxlive: 'link-xboxlive-oauth-base'
};

/**
 * @description The single Link/Unlink action for a platform row: starts
 * the OAuth linking flow if it isn't linked yet, or unlinks it if it is.
 * @param platform {string}
 */
function handleLinkAction(platform) {
    const action = document.getElementById(`link-${platform}-action`);
    if (!action) {
        return;
    }

    if (action.dataset.linked === 'true') {
        unlinkPlatform(platform);
        return;
    }

    const state = {
        platform: platform,
        nonce: createNonce(),
        redirect_uri: linkRedirect,
        mode: 'link'
    };

    if (platform === 'steam') {
        window.location.href = buildSteamOpenIDURL(state);
        return;
    }

    const base = document.getElementById(LINK_OAUTH_BASE_IDS[platform]);
    if (!base) {
        return;
    }
    window.location.href = base.innerText + '&state=' + encodeState(state);
}

/**
 * @description Guards per-platform against acting on a stale, superseded
 * link/unlink/toggle response.
 */
const platformActionSeq = {};

/**
 * @description Unlinks a platform from the caller's account after
 * confirmation, then refreshes the linked-accounts rows.
 * @param platform {string}
 */
function unlinkPlatform(platform) {
    if (!confirm(`Unlink ${platform} from your account?`)) {
        return;
    }

    const seq = (platformActionSeq[platform] || 0) + 1;
    platformActionSeq[platform] = seq;

    fetch(`${apiBaseUrl()}/api/v1/users/me/link/${platform}`, {
        method: 'DELETE',
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (res.status === 204) {
                loadLinkedAccounts();
                return;
            }
            return res.json().then((problem) => {
                throw new Error(problem.detail || 'Failed to unlink platform');
            });
        })
        .catch((error) => {
            if (platformActionSeq[platform] === seq) {
                alert(error.message);
            }
        });
}

/**
 * @description Sets whether a linked platform can be used to log in, from
 * its "Allow logins" checkbox. Reverts the checkbox if the request fails,
 * unless a newer link/unlink/toggle request for the same platform has
 * since been made.
 * @param platform {string}
 * @param enabled {boolean}
 */
function setPlatformLoginEnabled(platform, enabled) {
    const seq = (platformActionSeq[platform] || 0) + 1;
    platformActionSeq[platform] = seq;

    fetch(`${apiBaseUrl()}/api/v1/users/me/link/${platform}`, {
        method: 'PATCH',
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({login_enabled: enabled})
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (res.status === 204) {
                if (platformActionSeq[platform] === seq) {
                    loadLinkedAccounts();
                }
                return;
            }
            return res.json().then((problem) => {
                throw new Error(problem.detail || 'Failed to update platform');
            });
        })
        .catch((error) => {
            if (platformActionSeq[platform] === seq) {
                document.getElementById(`link-${platform}-login-enabled`).checked = !enabled;
                alert(error.message);
            }
        });
}

/**
 * @description Loads pending bee name suggestions into the admin page,
 * redirecting to /login if the session is missing or expired.
 */
function loadBeeSuggestions() {
    const error = document.getElementById('bee-admin-error');
    fetch(`${apiBaseUrl()}/api/v1/bee-name-generator/suggestion/100`, {
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (!res.ok) {
                return res.json().then((problem) => {
                    throw new Error(problem.detail || 'Failed to load suggestions');
                });
            }
            return res.json();
        })
        .then((data) => {
            if (!data) {
                return;
            }
            const list = document.getElementById('bee-suggestions');
            (data.suggestions || []).forEach((name) => list.appendChild(buildBeeSuggestionRow(name)));
            updateBeeSuggestionsEmptyState();
        })
        .catch((err) => {
            error.textContent = err.message;
            error.hidden = false;
        });
}

/**
 * @description Builds one suggestion row. Suggestions are user-submitted, so
 * the name is only ever set via textContent.
 * @param name {string}
 * @returns {HTMLLIElement}
 */
function buildBeeSuggestionRow(name) {
    const row = document.createElement('li');
    row.className = 'flex items-center justify-between gap-3 rounded-lg border border-input p-3';

    const label = document.createElement('span');
    label.className = 'min-w-0 flex-1 truncate text-sm font-medium';
    label.textContent = name;

    const accept = document.createElement('button');
    accept.className = 'shrink-0 rounded-md bg-primary px-3 py-1 text-xs font-medium text-primary-foreground hover:bg-primary-light';
    accept.textContent = 'Accept';
    accept.onclick = () => reviewBeeSuggestion(name, true, row);

    const reject = document.createElement('button');
    reject.className = 'shrink-0 rounded-md border border-input px-3 py-1 text-xs font-medium hover:bg-accent hover:text-accent-foreground';
    reject.textContent = 'Reject';
    reject.onclick = () => reviewBeeSuggestion(name, false, row);

    row.append(label, accept, reject);
    return row;
}

function updateBeeSuggestionsEmptyState() {
    const empty = document.getElementById('bee-suggestions-empty');
    empty.hidden = document.getElementById('bee-suggestions').children.length > 0;
}

/**
 * @description Accepts (PUT) or rejects (DELETE) a suggestion and removes
 * its row on success; alerts with the API's problem detail on failure.
 * @param name {string}
 * @param accept {boolean}
 * @param row {HTMLLIElement}
 */
function reviewBeeSuggestion(name, accept, row) {
    const buttons = row.querySelectorAll('button');
    buttons.forEach((b) => { b.disabled = true; });

    fetch(`${apiBaseUrl()}/api/v1/bee-name-generator/suggestion/${encodeURIComponent(name)}`, {
        method: accept ? 'PUT' : 'DELETE',
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (res.ok) {
                row.remove();
                updateBeeSuggestionsEmptyState();
                return;
            }
            return res.json().then((problem) => {
                throw new Error(problem.detail || 'Failed to update suggestion');
            });
        })
        .catch((err) => {
            buttons.forEach((b) => { b.disabled = false; });
            alert(err.message);
        });
}

/**
 * @description Reveals the bee name suggestion review link for accounts
 * holding the bee name admin permission. Cosmetic only - the admin
 * endpoints enforce the permission server-side.
 */
function showBeeAdminLink() {
    fetch(`${apiBaseUrl()}/api/v1/users/me/permissions`, {
        credentials: 'include'
    })
        .then((res) => (res.ok ? res.json() : null))
        .then((permissions) => {
            if ((permissions || []).includes('beenamegenerator|*')) {
                document.getElementById('bee-admin-link').hidden = false;
            }
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}

let mcStatusSeq = 0;

/**
 * @description Renders a server status response into the MC Status result
 * card. All server-supplied text goes in via textContent.
 * @param {Object} status - Server status from the mcstatus API
 */
function renderMcStatus(status) {
    const maxPlayers = status.max_players ?? 0;
    const numPlayers = status.num_players ?? 0;
    const players = status.players || [];

    document.getElementById('mc-status-name').textContent = status.name || status.host;
    document.getElementById('mc-status-motd').textContent = (status.motd || '').replace(/§[0-9a-fk-or]/gi, '').trim();
    document.getElementById('mc-status-version').textContent = status.version || 'Unknown version';
    document.getElementById('mc-status-type').textContent = status.server_type === 'bedrock' ? 'Bedrock' : 'Java';
    document.getElementById('mc-status-players-count').textContent = `${numPlayers} / ${maxPlayers}`;
    document.getElementById('mc-status-players-bar').style.width = maxPlayers > 0 ? `${Math.min(100, (numPlayers / maxPlayers) * 100)}%` : '0%';

    const icon = document.getElementById('mc-status-icon');
    const favicon = status.favicon || '';
    icon.hidden = !favicon.startsWith('data:image/png;base64,');
    icon.src = icon.hidden ? '' : favicon;

    const pill = document.getElementById('mc-status-pill');
    pill.textContent = 'Online';
    pill.className = 'rounded-full bg-green-500/15 px-2.5 py-0.5 text-xs font-medium text-green-600 dark:text-green-400';

    const list = document.getElementById('mc-status-players');
    list.replaceChildren();
    players.forEach((player) => {
        const chip = document.createElement('li');
        chip.className = 'rounded-full border border-input px-2.5 py-0.5 text-xs';
        chip.textContent = player.name;
        list.appendChild(chip);
    });
    document.getElementById('mc-status-players-hidden').hidden = players.length > 0 || numPlayers === 0;
    document.getElementById('mc-status-result').hidden = false;
}

/**
 * @description Shows an error in the MC Status result area in place of the
 * result card.
 * @param {string} message - Headline to show
 * @param {string} [detail] - Optional extra detail from the API
 */
function showMcStatusError(message, detail) {
    document.getElementById('mc-status-result').hidden = true;
    document.getElementById('mc-status-error-message').textContent = message;
    const detailEl = document.getElementById('mc-status-error-detail');
    detailEl.textContent = detail || '';
    detailEl.hidden = !detail;
    document.getElementById('mc-status-error').hidden = false;
}

/**
 * @description Looks up a server's status from the form values, keeps the
 * address bar in sync so the lookup can be shared, and renders the result.
 * @param {Event} [event] - Form submit event
 */
function checkMcStatus(event) {
    if (event) {
        event.preventDefault();
    }
    const host = document.getElementById('mc-status-host').value.trim();
    if (!host) {
        return;
    }
    const bedrock = document.querySelector('input[name="mc-edition"]:checked').value === 'bedrock';
    const query = !bedrock && document.getElementById('mc-status-query').checked;

    const params = new URLSearchParams({ host });
    if (bedrock) {
        params.set('bedrock', 'true');
    }
    if (query) {
        params.set('query', 'true');
    }
    history.replaceState(null, '', `${window.location.pathname}?${params}`);

    const apiParams = new URLSearchParams();
    if (bedrock) {
        apiParams.set('bedrock', 'true');
    }
    if (query) {
        apiParams.set('query', 'true');
    }

    const seq = ++mcStatusSeq;
    const button = document.getElementById('mc-status-submit');
    button.disabled = true;
    button.textContent = 'Checking...';
    document.getElementById('mc-status-error').hidden = true;

    fetch(`${apiBaseUrl()}/api/v1/mcstatus/${encodeURIComponent(host)}?${apiParams}`)
        .then((res) => {
            if (res.ok) {
                return res.json().then((status) => {
                    if (seq === mcStatusSeq) {
                        renderMcStatus(status);
                    }
                });
            }
            return res.json().catch(() => ({})).then((problem) => {
                if (seq !== mcStatusSeq) {
                    return;
                }
                if (res.status === 502) {
                    showMcStatusError("Couldn't reach that server", problem.detail);
                } else {
                    showMcStatusError('Something went wrong', problem.detail);
                }
            });
        })
        .catch((error) => {
            console.error('Error:', error);
            if (seq === mcStatusSeq) {
                showMcStatusError('Something went wrong', 'Check your connection and try again.');
            }
        })
        .finally(() => {
            if (seq === mcStatusSeq) {
                button.disabled = false;
                button.textContent = 'Check';
            }
        });
}

/**
 * @description Fills the MC Status form from the address bar and runs the
 * lookup when a host is present, so shared links open on their result.
 */
function loadMcStatusFromUrl() {
    const params = new URLSearchParams(window.location.search);
    const host = params.get('host');
    if (!host) {
        return;
    }
    document.getElementById('mc-status-host').value = host;
    const edition = params.get('bedrock') === 'true' ? 'bedrock' : 'java';
    document.querySelector(`input[name="mc-edition"][value="${edition}"]`).checked = true;
    document.getElementById('mc-status-query').checked = params.get('query') === 'true';
    checkMcStatus();
}
