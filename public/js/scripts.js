function apiBaseUrl() {
    return document.getElementById('api-base-url').innerText;
}

/** The API encodes the problem with Go's base64.URLEncoding. */
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

function steamOpenIdLoginUrl() {
    return document.getElementById('steam-openid-login-url').innerText;
}

function sharedHostSuffix(siteHost, apiHost) {
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
 * A Domain the browser rejects (a public suffix like co.uk) drops the cookie
 * silently, so confirm it stuck and fall back to a host-only cookie.
 * SameSite=None requires Secure, so plain http (local dev) falls back to Lax.
 */
function setNonceCookie(nonce, domain) {
    const secureAttr = location.protocol === 'https:' ? '; Secure' : '';
    const sameSite = secureAttr ? 'None' : 'Lax';
    const expires = new Date(Date.now() + 5 * 60 * 1000).toUTCString();
    const attrs = `; expires=${expires}; path=/; SameSite=${sameSite}${secureAttr}`;
    if (domain) {
        document.cookie = `nonce=${nonce}; domain=${domain}${attrs}`;
        if (document.cookie.split('; ').includes(`nonce=${nonce}`)) {
            return;
        }
    }
    document.cookie = `nonce=${nonce}${attrs}`;
}

function createNonce() {
    const nonce = Math.random().toString(36).substring(2, 15);
    setNonceCookie(nonce, sharedHostSuffix(location.hostname, new URL(apiBaseUrl()).hostname));
    return nonce;
}

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


/** base64url - the API decodes state with Go's base64.URLEncoding. */
function encodeState(state) {
    return btoa(JSON.stringify(state)).replace(/\+/g, '-').replace(/\//g, '_');
}

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

const LINK_OAUTH_BASE_IDS = {
    discord: 'link-discord-oauth-base',
    twitch: 'link-twitch-oauth-base',
    microsoft: 'link-microsoft-oauth-base',
    xboxlive: 'link-xboxlive-oauth-base'
};

function handleLinkAction(platform) {
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

/** "." and ".." are collapsed out of URL paths by fetch, so they can't be sent as a path segment. */
function isDotSegment(name) {
    return name === '.' || name === '..';
}

const MC_STATUS_PATH = '/project/mc-status';
const MC_STATUS_TIMEOUT_MS = 30000;
let mcStatusController = null;

/** The API sends line breaks as a literal backslash-n. */
function formatMcMotd(motd) {
    return motd
        .replace(/\\n/g, '\n')
        .replace(/§[^]/giu, '')
        .trim();
}

/** Server-supplied text goes in via textContent only. */
function renderMcStatus(status, bedrock, host) {
    const maxPlayers = status.max_players ?? 0;
    const numPlayers = status.num_players ?? 0;
    const players = status.players || [];

    document.getElementById('mc-status-name').textContent = host;
    document.getElementById('mc-status-motd').textContent = formatMcMotd(status.motd || '');
    document.getElementById('mc-status-version').textContent = status.version || 'Unknown version';
    document.getElementById('mc-status-type').textContent = bedrock ? 'Bedrock' : 'Java';
    document.getElementById('mc-status-players-count').textContent = `${numPlayers} / ${maxPlayers}`;

    const bar = document.getElementById('mc-status-players-bar');
    bar.style.width = maxPlayers > 0 ? `${Math.min(100, (numPlayers / maxPlayers) * 100)}%` : '0%';
    const track = bar.parentElement;
    track.setAttribute('aria-valuemax', maxPlayers);
    track.setAttribute('aria-valuenow', Math.min(numPlayers, maxPlayers));

    const icon = document.getElementById('mc-status-icon');
    icon.hidden = true;
    icon.onload = () => { icon.hidden = false; };
    icon.removeAttribute('src');
    icon.src = `${apiBaseUrl()}/api/v1/mcstatus/icon/${encodeURIComponent(host)}${bedrock ? '?bedrock=true' : ''}`;

    const pill = document.getElementById('mc-status-pill');
    pill.textContent = 'Online';
    pill.className = 'rounded-full bg-green-500/15 px-2.5 py-0.5 text-xs font-medium text-green-600 dark:text-green-400';

    const chipClass = 'max-w-full truncate rounded-full border border-input px-2.5 py-0.5 text-xs';
    const list = document.getElementById('mc-status-players');
    list.replaceChildren();
    players.forEach((player) => {
        const chip = document.createElement('li');
        chip.className = chipClass;
        chip.textContent = player.name;
        list.appendChild(chip);
    });
    if (players.length > 0 && numPlayers > players.length) {
        const more = document.createElement('li');
        more.className = chipClass;
        more.textContent = `and ${numPlayers - players.length} more`;
        list.appendChild(more);
    }
    document.getElementById('mc-status-players-unavailable').hidden = players.length > 0 || numPlayers === 0;
    document.getElementById('mc-status-error').hidden = true;
    document.getElementById('mc-status-result').hidden = false;
}

function showMcStatusError(message, detail) {
    document.getElementById('mc-status-result').hidden = true;
    document.getElementById('mc-status-error-message').textContent = message;
    const detailEl = document.getElementById('mc-status-error-detail');
    detailEl.textContent = detail || '';
    detailEl.hidden = !detail;
    document.getElementById('mc-status-error').hidden = false;
}

let mcQueryPreference = true;

function syncMcStatusQueryOption() {
    const bedrock = document.querySelector('input[name="mc-edition"]:checked').value === 'bedrock';
    const query = document.getElementById('mc-status-query');
    query.disabled = bedrock;
    query.checked = !bedrock && mcQueryPreference;
    document.getElementById('mc-status-query-port').disabled = !query.checked;
}

const MC_MAX_PORT = 65535;

function parseMcPort(value) {
    const port = /^\d+$/.test(value) ? Number(value) : 0;
    return port >= 1 && port <= MC_MAX_PORT ? port : null;
}

/** Must match McStatusEmbedData.DisplayHost in components/mcstatus_embed.templ, or the shared URL stops round-tripping. */
function mcDisplayHost(target, bedrock) {
    const host = target.host.includes(':') ? `[${target.host}]` : target.host;
    return target.port === (bedrock ? 19132 : 25565) ? host : `${host}:${target.port}`;
}

function setMcStatusUrl(host, search) {
    const path = `${MC_STATUS_PATH}/${encodeURIComponent(host).replaceAll('%3A', ':')}`;
    history.replaceState(null, '', search ? `${path}?${search}` : path);
}

function validateMcQueryPort() {
    const input = document.getElementById('mc-status-query-port');
    input.setCustomValidity(input.value === '' || parseMcPort(input.value) !== null ? '' : `Enter a port from 1 to ${MC_MAX_PORT}`);
}

function mcStatusQueryPort() {
    const input = document.getElementById('mc-status-query-port');
    return input.disabled ? null : parseMcPort(input.value);
}

function abortMcStatus() {
    if (mcStatusController) {
        mcStatusController.abort();
        mcStatusController = null;
    }
    document.getElementById('mc-status-submit').textContent = 'Check';
}

function checkMcStatus(event) {
    if (event) {
        event.preventDefault();
    }
    abortMcStatus();
    const host = document.getElementById('mc-status-host').value.trim();
    if (!host || isDotSegment(host)) {
        showMcStatusError('Enter a valid server address');
        return;
    }
    const bedrock = document.querySelector('input[name="mc-edition"]:checked').value === 'bedrock';
    const query = !bedrock && document.getElementById('mc-status-query').checked;
    const queryPort = query ? mcStatusQueryPort() : null;

    const params = new URLSearchParams();
    if (bedrock) {
        params.set('bedrock', 'true');
    }
    if (query) {
        params.set('query', 'true');
        if (queryPort) {
            params.set('query_port', queryPort);
        }
    }
    const urlParams = new URLSearchParams();
    if (bedrock) {
        urlParams.set('bedrock', 'true');
    } else if (!query) {
        urlParams.set('query', 'false');
    } else if (queryPort) {
        urlParams.set('query_port', queryPort);
    }
    const search = urlParams.toString();

    const controller = new AbortController();
    mcStatusController = controller;
    let timedOut = false;
    const timeout = setTimeout(() => {
        timedOut = true;
        controller.abort();
    }, MC_STATUS_TIMEOUT_MS);

    const button = document.getElementById('mc-status-submit');
    button.textContent = 'Checking...';
    document.getElementById('mc-status-result').hidden = true;
    document.getElementById('mc-status-error').hidden = true;

    fetch(`${apiBaseUrl()}/api/v1/mcstatus/${encodeURIComponent(host)}?${params}`, { signal: controller.signal })
        .then((res) => {
            if (res.ok) {
                return res.json().then((status) => {
                    if (controller === mcStatusController) {
                        const canonicalHost = mcDisplayHost(status, bedrock);
                        renderMcStatus(status, bedrock, canonicalHost);
                        setMcStatusUrl(canonicalHost, search);
                    }
                });
            }
            return res.json().catch(() => ({})).then((problem) => {
                if (controller !== mcStatusController) {
                    return;
                }
                if (timedOut) {
                    showMcStatusError("Couldn't reach that server", 'The lookup timed out.');
                    return;
                }
                const detail = problem.detail || '';
                if (res.status === 404 && (res.headers.get('content-type') || '').split(';')[0].trim().toLowerCase() === 'application/problem+json') {
                    showMcStatusError("Couldn't reach that server", detail);
                    if (problem.host && problem.port) {
                        setMcStatusUrl(mcDisplayHost(problem, bedrock), search);
                    }
                } else if (res.status === 400) {
                    history.replaceState(null, '', MC_STATUS_PATH);
                    showMcStatusError('Enter a valid server address', detail);
                } else if (res.status === 429) {
                    showMcStatusError('Too many lookups', 'Please try again in a minute.');
                } else if (res.status === 500) {
                    showMcStatusError('Something went wrong', detail);
                } else {
                    showMcStatusError(`Unexpected response (${res.status})`, detail);
                }
            });
        })
        .catch((error) => {
            if (controller !== mcStatusController) {
                return;
            }
            if (timedOut) {
                showMcStatusError("Couldn't reach that server", 'The lookup timed out.');
                return;
            }
            console.error('Error:', error);
            showMcStatusError('Something went wrong', 'Check your connection and try again.');
        })
        .finally(() => {
            clearTimeout(timeout);
            if (controller === mcStatusController) {
                button.textContent = 'Check';
            }
        });
}

function loadMcStatusFromUrl() {
    const params = new URLSearchParams(window.location.search);
    let host = '';
    if (window.location.pathname.startsWith(`${MC_STATUS_PATH}/`)) {
        try {
            host = decodeURIComponent(window.location.pathname.slice(MC_STATUS_PATH.length + 1));
        } catch {
            return;
        }
    }
    if (!host) {
        return;
    }
    document.getElementById('mc-status-host').value = host;
    const edition = params.get('bedrock') === 'true' ? 'bedrock' : 'java';
    document.querySelector(`input[name="mc-edition"][value="${edition}"]`).checked = true;
    mcQueryPreference = params.get('query') !== 'false';
    const queryPort = parseMcPort(params.get('query_port') || '');
    document.getElementById('mc-status-query-port').value = queryPort || '';
    document.getElementById('mc-status-advanced').open = queryPort !== null;
    syncMcStatusQueryOption();
    checkMcStatus();
}

/** htmx reports a failed, a timed-out and an aborted request alike as htmx:error, and a search replaced by a newer one is an abort that is not a failure. */
document.addEventListener('htmx:error', (event) => {
    const { ctx, error } = event.detail;
    const replaced = event.target.getAttribute?.('hx-sync')?.endsWith(':replace');
    const failed = error instanceof TypeError || (error?.name === 'AbortError' && !replaced);
    if (ctx && !ctx.response && failed) {
        const banner = document.getElementById('admin-error');
        if (banner) {
            banner.textContent = 'The server could not be reached. Try again in a moment.';
        }
    }
});
