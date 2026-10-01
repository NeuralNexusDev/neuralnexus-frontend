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

let passwordAuthSeq = 0;

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

const LINK_PLATFORMS = ['discord', 'twitch', 'microsoft', 'xboxlive', 'steam'];

let loadLinkedAccountsSeq = 0;

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

const LINK_OAUTH_BASE_IDS = {
    discord: 'link-discord-oauth-base',
    twitch: 'link-twitch-oauth-base',
    microsoft: 'link-microsoft-oauth-base',
    xboxlive: 'link-xboxlive-oauth-base'
};

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

const platformActionSeq = {};

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

/** "." and ".." are collapsed out of URL paths by fetch, so they can't be sent as a path segment. */
function isDotSegment(name) {
    return name === '.' || name === '..';
}

function problemDetail(res, fallback) {
    return res.json().catch(() => ({})).then((problem) => problem.detail || fallback);
}

function loadBeeSuggestions() {
    const error = document.getElementById('bee-admin-error');
    const list = document.getElementById('bee-suggestions');
    error.hidden = true;
    fetch(`${apiBaseUrl()}/api/v1/bee-name-generator/suggestion/100`, {
        credentials: 'include'
    })
        .then((res) => {
            if (res.status === 401) {
                window.location.href = '/login';
                return;
            }
            if (!res.ok) {
                return problemDetail(res, 'Failed to load suggestions').then((detail) => {
                    throw new Error(detail);
                });
            }
            return res.json();
        })
        .then((data) => {
            if (!data) {
                return;
            }
            list.replaceChildren(...(data.suggestions || []).map(buildBeeSuggestionRow));
            updateBeeSuggestionsEmptyState();
        })
        .catch((err) => {
            error.textContent = err.message;
            error.hidden = false;
        });
}

/** Suggestions are user-submitted - set the name via textContent only. */
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

function reviewBeeSuggestion(name, accept, row) {
    if (isDotSegment(name)) {
        alert('Names made only of dots can\'t be reviewed from the browser.');
        return;
    }
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
                if (document.getElementById('bee-suggestions').children.length === 0) {
                    loadBeeSuggestions();
                }
                return;
            }
            return problemDetail(res, 'Failed to update suggestion').then((detail) => {
                throw new Error(detail);
            });
        })
        .catch((err) => {
            buttons.forEach((b) => { b.disabled = false; });
            alert(err.message);
        });
}

/** Cosmetic only - the admin endpoints enforce the permission. */
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
const MC_MAX_HOSTNAME = 253;
const MC_HOST_PATTERN = /^[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)*(?::([0-9]{1,5}))?$/;

function parseMcPort(value) {
    const port = /^\d+$/.test(value) ? Number(value) : 0;
    return port >= 1 && port <= MC_MAX_PORT ? port : null;
}

/** The shareable path for a host, normalised the way the server does; the bare path when the server would reject it. */
function mcStatusPath(host) {
    const match = MC_HOST_PATTERN.exec(host);
    if (!match) {
        return MC_STATUS_PATH;
    }
    const portText = match[1];
    const name = portText === undefined ? host : host.slice(0, -portText.length - 1);
    const port = portText === undefined ? null : parseMcPort(portText);
    if (name.length > MC_MAX_HOSTNAME || (portText !== undefined && port === null)) {
        return MC_STATUS_PATH;
    }
    return `${MC_STATUS_PATH}/${name.toLowerCase()}${port === null ? '' : `:${port}`}`;
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
    const path = mcStatusPath(host);
    const search = path === MC_STATUS_PATH ? '' : urlParams.toString();
    history.replaceState(null, '', search ? `${path}?${search}` : path);

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
                        renderMcStatus(status, bedrock, host);
                    }
                });
            }
            return problemDetail(res, '').then((detail) => {
                if (controller !== mcStatusController) {
                    return;
                }
                if (timedOut) {
                    showMcStatusError("Couldn't reach that server", 'The lookup timed out.');
                    return;
                }
                if (res.status === 404 && (res.headers.get('content-type') || '').split(';')[0].trim().toLowerCase() === 'application/problem+json') {
                    showMcStatusError("Couldn't reach that server", detail);
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
            host = decodeURIComponent(window.location.pathname.slice(MC_STATUS_PATH.length + 1).replace(/\/$/, ''));
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
