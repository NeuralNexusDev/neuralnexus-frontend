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

function adminRequest(path, options) {
    return fetch(`${apiBaseUrl()}/api/v1${path}`, { credentials: 'include', ...options }).then((res) => {
        if (res.status === 401) {
            window.location.href = '/login';
        }
        return res;
    });
}

/** Cosmetic only - the admin endpoints enforce the permission. */
function hasAdminPermission(permissions, node) {
    return permissions.some((permission) => permission === node || permission.startsWith(`${node}:`));
}

function showAdminError(message) {
    const error = document.getElementById('admin-error');
    error.textContent = message;
    error.hidden = false;
}

function showAdminProblem(res, fallback) {
    return problemDetail(res, fallback).then(showAdminError);
}

function showAdminDashboardLink() {
    adminRequest('/users/me/permissions')
        .then((res) => (res.ok ? res.json() : null))
        .then((permissions) => {
            if (permissions && (hasAdminPermission(permissions, 'users.admin') || hasAdminPermission(permissions, 'roles.admin'))) {
                document.getElementById('admin-dashboard-link').hidden = false;
            }
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}

function loadAdminDashboard() {
    adminRequest('/users/me/permissions')
        .then((res) => {
            if (!res.ok) {
                return showAdminProblem(res, 'Failed to load your permissions');
            }
            return res.json().then((permissions) => {
                const users = hasAdminPermission(permissions, 'users.admin');
                const roles = hasAdminPermission(permissions, 'roles.admin');
                document.getElementById('admin-users-link').hidden = !users;
                document.getElementById('admin-roles-link').hidden = !roles;
                document.getElementById('admin-permissions-link').hidden = !roles;
                document.getElementById('admin-denied').hidden = users || roles;
            });
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load your permissions');
        });
}

/** Role names and descriptions by ID, or null when the caller lacks roles.admin. */
function loadAdminRoles() {
    return adminRequest('/roles').then((res) => {
        if (!res.ok) {
            return null;
        }
        return res.json().then((roles) => new Map(roles.map((role) => [role.id, role])));
    });
}

const ADMIN_USERS_PAGE_SIZE = 200;
let adminUsers = [];
let adminUsersRoles = null;

function fetchAdminUsersPage() {
    return adminRequest(`/users?limit=${ADMIN_USERS_PAGE_SIZE}&offset=${adminUsers.length}`).then((res) => {
        if (!res.ok) {
            return showAdminProblem(res, 'Failed to load users').then(() => null);
        }
        return res.json();
    });
}

function addAdminUsersPage(page) {
    if (page === null) {
        return;
    }
    adminUsers = adminUsers.concat(page);
    document.getElementById('admin-users-more').hidden = page.length < ADMIN_USERS_PAGE_SIZE;
    renderAdminUsers();
}

function loadAdminUsers() {
    Promise.all([fetchAdminUsersPage(), loadAdminRoles()])
        .then(([page, roles]) => {
            adminUsersRoles = roles;
            addAdminUsersPage(page);
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load users');
        });
}

function loadMoreAdminUsers() {
    runAdminAction(document.getElementById('admin-users-more-button'), () => fetchAdminUsersPage().then(addAdminUsersPage), 'Failed to load users');
}

function adminRoleName(roles, id) {
    return roles && roles.has(id) ? roles.get(id).name : id;
}

function renderAdminUsers() {
    const search = document.getElementById('admin-users-search').value.trim().toLowerCase();
    const matches = adminUsers.filter((user) => (user.username || '').toLowerCase().includes(search) || user.user_id.includes(search));
    const list = document.getElementById('admin-users');
    list.replaceChildren();
    matches.forEach((user) => {
        const item = document.createElement('li');
        const link = document.createElement('a');
        link.href = `/admin/users/${encodeURIComponent(user.user_id)}`;
        link.className = 'block rounded-lg border border-input p-3 transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';
        const name = document.createElement('span');
        name.className = 'block font-medium';
        name.textContent = user.username || 'No username';
        const id = document.createElement('span');
        id.className = 'block break-all text-xs text-gray-500 dark:text-gray-400';
        id.textContent = user.user_id;
        link.append(name, id);
        const roles = user.roles || [];
        if (roles.length > 0) {
            const chips = document.createElement('span');
            chips.className = 'mt-2 flex flex-wrap gap-2';
            roles.forEach((roleId) => {
                const chip = document.createElement('span');
                chip.className = 'rounded-full border border-input px-2.5 py-0.5 text-xs';
                chip.textContent = adminRoleName(adminUsersRoles, roleId);
                chips.appendChild(chip);
            });
            link.appendChild(chips);
        }
        item.appendChild(link);
        list.appendChild(item);
    });
    document.getElementById('admin-users-empty').hidden = matches.length > 0;
}

let adminUserId = '';
let adminUserRolesEditable = false;

function adminUserIdFromPath() {
    try {
        return decodeURIComponent(window.location.pathname.slice('/admin/users/'.length));
    } catch {
        return '';
    }
}

function loadAdminUser() {
    adminUserId = adminUserIdFromPath();
    document.getElementById('admin-user-id').textContent = adminUserId;
    const id = encodeURIComponent(adminUserId);
    Promise.all([adminRequest(`/users/${id}`), adminRequest(`/users/${id}/links`), adminRequest(`/users/${id}/permissions`), loadAdminRoles()])
        .then(([userRes, linksRes, permissionsRes, roles]) => {
            if (!userRes.ok) {
                return showAdminProblem(userRes, 'Failed to load the user');
            }
            return Promise.all([
                userRes.json(),
                linksRes.ok ? linksRes.json() : [],
                permissionsRes.ok ? permissionsRes.json() : [],
            ]).then(([user, links, permissions]) => {
                renderAdminUser(user, roles);
                renderAdminUserLinks(links || []);
                renderAdminUserPermissions(permissions || []);
                document.getElementById('admin-user-form').hidden = false;
            });
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load the user');
        });
}

function renderAdminUser(user, roles) {
    document.getElementById('admin-user-title').textContent = user.username || 'No username';
    document.getElementById('admin-user-username').value = user.username || '';
    adminUserRolesEditable = roles !== null;
    document.getElementById('admin-user-roles-note').hidden = adminUserRolesEditable;
    const held = new Set(user.roles || []);
    const list = document.getElementById('admin-user-roles');
    list.replaceChildren();
    if (!adminUserRolesEditable) {
        held.forEach((roleId) => {
            const item = document.createElement('li');
            item.className = 'rounded-md border border-input px-3 py-2 text-sm';
            item.textContent = roleId;
            list.appendChild(item);
        });
        return;
    }
    roles.forEach((role) => {
        const item = document.createElement('li');
        const label = document.createElement('label');
        label.className = 'flex cursor-pointer items-start gap-3 rounded-md border border-input px-3 py-2 text-sm';
        const box = document.createElement('input');
        box.type = 'checkbox';
        box.value = role.id;
        box.checked = held.has(role.id);
        box.className = 'mt-1';
        const text = document.createElement('span');
        const name = document.createElement('span');
        name.className = 'block font-medium';
        name.textContent = role.name;
        const description = document.createElement('span');
        description.className = 'block text-xs text-gray-500 dark:text-gray-400';
        description.textContent = role.description;
        text.append(name, description);
        label.append(box, text);
        item.appendChild(label);
        list.appendChild(item);
    });
}

function renderAdminUserLinks(links) {
    const list = document.getElementById('admin-user-links');
    list.replaceChildren();
    links.forEach((link) => {
        const item = document.createElement('li');
        item.className = 'flex justify-between gap-3 rounded-md border border-input px-3 py-2';
        const platform = document.createElement('span');
        platform.className = 'font-medium capitalize';
        platform.textContent = link.platform;
        const username = document.createElement('span');
        username.className = 'min-w-0 truncate text-gray-500 dark:text-gray-400';
        username.textContent = link.platform_username || link.platform_id || '';
        item.append(platform, username);
        list.appendChild(item);
    });
    document.getElementById('admin-user-links-empty').hidden = links.length > 0;
}

function renderAdminUserPermissions(permissions) {
    const list = document.getElementById('admin-user-permissions');
    list.replaceChildren();
    permissions.forEach((permission) => {
        const chip = document.createElement('li');
        chip.className = 'rounded-full border border-input px-2.5 py-0.5 text-xs';
        chip.textContent = permission;
        list.appendChild(chip);
    });
    document.getElementById('admin-user-permissions-empty').hidden = permissions.length > 0;
}

function saveAdminUser(event) {
    event.preventDefault();
    const body = { username: document.getElementById('admin-user-username').value.trim() };
    if (adminUserRolesEditable) {
        body.roles = [...document.querySelectorAll('#admin-user-roles input:checked')].map((box) => box.value);
    }
    const save = document.getElementById('admin-user-save');
    const status = document.getElementById('admin-user-status');
    document.getElementById('admin-error').hidden = true;
    status.hidden = true;
    save.disabled = true;
    adminRequest(`/users/${encodeURIComponent(adminUserId)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    })
        .then((res) => {
            if (!res.ok) {
                return showAdminProblem(res, 'Failed to save the user');
            }
            return res.json().then((user) => {
                document.getElementById('admin-user-title').textContent = user.username || 'No username';
                status.textContent = 'Saved';
                status.hidden = false;
                return adminRequest(`/users/${encodeURIComponent(adminUserId)}/permissions`)
                    .then((permissionsRes) => (permissionsRes.ok ? permissionsRes.json() : null))
                    .then((permissions) => {
                        if (permissions) {
                            renderAdminUserPermissions(permissions);
                        }
                    });
            });
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to save the user');
        })
        .finally(() => {
            save.disabled = false;
        });
}

function adminJSON(method, path, body) {
    return adminRequest(path, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
}

function runAdminAction(button, action, fallback) {
    document.getElementById('admin-error').hidden = true;
    button.disabled = true;
    return action()
        .catch((error) => {
            console.error('Error:', error);
            showAdminError(fallback);
        })
        .finally(() => {
            button.disabled = false;
        });
}

const ADMIN_INPUT_CLASS = 'text-foreground flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2';
const ADMIN_BUTTON_CLASS = 'border border-input hover:bg-accent hover:text-accent-foreground inline-flex h-9 items-center justify-center whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50';
const ADMIN_CHIP_CLASS = 'rounded-full border border-input px-2.5 py-0.5 text-xs';

function adminPermissionLabel(permission) {
    if (permission.value === undefined) {
        return permission.node;
    }
    return `${permission.node}: ${Array.isArray(permission.value) ? permission.value.join(', ') : permission.value}`;
}

/** The input for a permission's value, or null for a permission granted as is. */
function adminValueField(permission, value) {
    if (!permission.value_type) {
        return null;
    }
    if (permission.value_type === 'string_list') {
        const field = document.createElement('textarea');
        field.rows = 3;
        field.placeholder = 'One item per line';
        field.value = Array.isArray(value) ? value.join('\n') : '';
        field.className = `${ADMIN_INPUT_CLASS} h-auto`;
        return field;
    }
    const field = document.createElement('input');
    field.type = permission.value_type === 'int' ? 'number' : 'text';
    if (permission.value_type === 'int') {
        field.step = '1';
    }
    field.autocomplete = 'off';
    field.value = value === undefined ? '' : String(value);
    field.className = ADMIN_INPUT_CLASS;
    return field;
}

/** The request body for a permission's value field; throws the message to show when the input is unusable. */
function adminValueBody(permission, field) {
    if (!field) {
        return undefined;
    }
    if (permission.value_type === 'int') {
        if (field.value.trim() === '' || !Number.isInteger(Number(field.value))) {
            throw new Error('Enter a whole number');
        }
        return { value: Number(field.value) };
    }
    if (permission.value_type === 'string_list') {
        const items = field.value.split('\n').map((item) => item.trim()).filter((item) => item !== '');
        if (items.length === 0) {
            throw new Error('Enter at least one item');
        }
        return { value: items };
    }
    if (field.value.trim() === '') {
        throw new Error('Enter a value');
    }
    return { value: field.value.trim() };
}

function loadAdminRolesPage() {
    adminRequest('/roles')
        .then((res) => {
            if (!res.ok) {
                return showAdminProblem(res, 'Failed to load roles');
            }
            return res.json().then(renderAdminRoles);
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load roles');
        });
}

function renderAdminRoles(roles) {
    const list = document.getElementById('admin-roles');
    list.replaceChildren();
    roles.forEach((role) => {
        const item = document.createElement('li');
        const link = document.createElement('a');
        link.href = `/admin/roles/${encodeURIComponent(role.id)}`;
        link.className = 'block rounded-lg border border-input p-3 transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';
        const name = document.createElement('span');
        name.className = 'block font-medium';
        name.textContent = role.name;
        const description = document.createElement('span');
        description.className = 'block text-xs text-gray-500 dark:text-gray-400';
        description.textContent = role.description;
        link.append(name, description);
        const permissions = role.permissions || [];
        if (permissions.length > 0) {
            const chips = document.createElement('span');
            chips.className = 'mt-2 flex flex-wrap gap-2';
            permissions.forEach((permission) => {
                const chip = document.createElement('span');
                chip.className = ADMIN_CHIP_CLASS;
                chip.textContent = adminPermissionLabel(permission);
                chips.appendChild(chip);
            });
            link.appendChild(chips);
        }
        item.appendChild(link);
        list.appendChild(item);
    });
    document.getElementById('admin-roles-empty').hidden = roles.length > 0;
}

function createAdminRole(event) {
    event.preventDefault();
    runAdminAction(
        document.getElementById('admin-role-create-submit'),
        () =>
            adminJSON('POST', '/roles', {
                name: document.getElementById('admin-role-create-name').value.trim(),
                description: document.getElementById('admin-role-create-description').value.trim(),
            }).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to create the role');
                }
                return res.json().then((role) => {
                    window.location.href = `/admin/roles/${encodeURIComponent(role.id)}`;
                });
            }),
        'Failed to create the role'
    );
}

let adminRoleId = '';
let adminRole = null;
let adminAllPermissions = [];

function loadAdminRole() {
    try {
        adminRoleId = decodeURIComponent(window.location.pathname.slice('/admin/roles/'.length));
    } catch {
        adminRoleId = '';
    }
    document.getElementById('admin-role-id').textContent = adminRoleId;
    Promise.all([adminRequest(`/roles/${encodeURIComponent(adminRoleId)}`), adminRequest('/permissions')])
        .then(([roleRes, permissionsRes]) => {
            if (!roleRes.ok) {
                return showAdminProblem(roleRes, 'Failed to load the role');
            }
            if (!permissionsRes.ok) {
                return showAdminProblem(permissionsRes, 'Failed to load permissions');
            }
            return Promise.all([roleRes.json(), permissionsRes.json()]).then(([role, permissions]) => {
                adminAllPermissions = permissions;
                renderAdminRole(role);
                document.getElementById('admin-role-content').hidden = false;
            });
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load the role');
        });
}

function refreshAdminRole() {
    return adminRequest(`/roles/${encodeURIComponent(adminRoleId)}`).then((res) => {
        if (!res.ok) {
            return showAdminProblem(res, 'Failed to load the role');
        }
        return res.json().then(renderAdminRole);
    });
}

function renderAdminRole(role) {
    adminRole = role;
    document.getElementById('admin-role-title').textContent = role.name;
    document.getElementById('admin-role-name').value = role.name;
    document.getElementById('admin-role-description').value = role.description;

    const granted = role.permissions || [];
    const list = document.getElementById('admin-role-permissions');
    list.replaceChildren();
    granted.forEach((permission) => {
        const item = document.createElement('li');
        item.className = 'space-y-2 rounded-lg border border-input p-3';
        item.dataset.permission = permission.node;
        const header = document.createElement('div');
        header.className = 'flex items-start justify-between gap-3';
        const text = document.createElement('span');
        text.className = 'min-w-0';
        const node = document.createElement('span');
        node.className = 'block break-all font-medium';
        node.textContent = permission.node;
        const description = document.createElement('span');
        description.className = 'block text-xs text-gray-500 dark:text-gray-400';
        description.textContent = permission.description;
        text.append(node, description);
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = ADMIN_BUTTON_CLASS;
        remove.textContent = 'Remove';
        remove.onclick = () => removeAdminRolePermission(permission, remove);
        header.append(text, remove);
        item.appendChild(header);
        const field = adminValueField(permission, permission.value);
        if (field) {
            const row = document.createElement('div');
            row.className = 'flex items-start gap-2';
            field.setAttribute('aria-label', `Value of ${permission.node}`);
            const save = document.createElement('button');
            save.type = 'button';
            save.className = ADMIN_BUTTON_CLASS;
            save.textContent = 'Save value';
            save.onclick = () => putAdminRolePermission(permission, field, save);
            row.append(field, save);
            item.appendChild(row);
        }
        list.appendChild(item);
    });
    document.getElementById('admin-role-permissions-empty').hidden = granted.length > 0;

    const heldIds = new Set(granted.map((permission) => permission.id));
    const select = document.getElementById('admin-role-grant-permission');
    select.replaceChildren();
    adminAllPermissions
        .filter((permission) => !heldIds.has(permission.id))
        .forEach((permission) => {
            const option = document.createElement('option');
            option.value = permission.id;
            option.textContent = permission.node;
            select.appendChild(option);
        });
    const available = select.options.length > 0;
    document.getElementById('admin-role-grant-form').hidden = !available;
    document.getElementById('admin-role-grant-empty').hidden = available;
    renderAdminGrantValue();
}

function renderAdminGrantValue() {
    const container = document.getElementById('admin-role-grant-value');
    container.replaceChildren();
    const permission = adminAllPermissions.find((candidate) => candidate.id === document.getElementById('admin-role-grant-permission').value);
    const field = permission ? adminValueField(permission) : null;
    if (field) {
        field.setAttribute('aria-label', `Value of ${permission.node}`);
        container.appendChild(field);
    }
}

function putAdminRolePermission(permission, field, button) {
    let body;
    try {
        body = adminValueBody(permission, field);
    } catch (error) {
        showAdminError(error.message);
        return;
    }
    const path = `/roles/${encodeURIComponent(adminRoleId)}/permissions/${encodeURIComponent(permission.id)}`;
    runAdminAction(
        button,
        () =>
            (body ? adminJSON('PUT', path, body) : adminRequest(path, { method: 'PUT' })).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to grant the permission');
                }
                return refreshAdminRole();
            }),
        'Failed to grant the permission'
    );
}

function grantAdminRolePermission(event) {
    event.preventDefault();
    const permission = adminAllPermissions.find((candidate) => candidate.id === document.getElementById('admin-role-grant-permission').value);
    const field = document.querySelector('#admin-role-grant-value input, #admin-role-grant-value textarea');
    putAdminRolePermission(permission, field, document.getElementById('admin-role-grant-submit'));
}

function removeAdminRolePermission(permission, button) {
    runAdminAction(
        button,
        () =>
            adminRequest(`/roles/${encodeURIComponent(adminRoleId)}/permissions/${encodeURIComponent(permission.id)}`, { method: 'DELETE' }).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to remove the permission');
                }
                return refreshAdminRole();
            }),
        'Failed to remove the permission'
    );
}

function saveAdminRole(event) {
    event.preventDefault();
    const status = document.getElementById('admin-role-status');
    status.hidden = true;
    runAdminAction(
        document.getElementById('admin-role-save'),
        () =>
            adminJSON('PATCH', `/roles/${encodeURIComponent(adminRoleId)}`, {
                name: document.getElementById('admin-role-name').value.trim(),
                description: document.getElementById('admin-role-description').value.trim(),
            }).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to save the role');
                }
                return refreshAdminRole().then(() => {
                    status.textContent = 'Saved';
                    status.hidden = false;
                });
            }),
        'Failed to save the role'
    );
}

function deleteAdminRole() {
    if (!window.confirm(`Delete the role ${adminRole.name}?`)) {
        return;
    }
    runAdminAction(
        document.getElementById('admin-role-delete'),
        () =>
            adminRequest(`/roles/${encodeURIComponent(adminRoleId)}`, { method: 'DELETE' }).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to delete the role');
                }
                window.location.href = '/admin/roles';
            }),
        'Failed to delete the role'
    );
}

function loadAdminPermissions() {
    return adminRequest('/permissions')
        .then((res) => {
            if (!res.ok) {
                return showAdminProblem(res, 'Failed to load permissions');
            }
            return res.json().then(renderAdminPermissions);
        })
        .catch((error) => {
            console.error('Error:', error);
            showAdminError('Failed to load permissions');
        });
}

function renderAdminPermissions(permissions) {
    const list = document.getElementById('admin-permissions');
    list.replaceChildren();
    permissions.forEach((permission) => {
        const item = document.createElement('li');
        item.className = 'flex items-start justify-between gap-3 rounded-lg border border-input p-3';
        item.dataset.permission = permission.node;
        const text = document.createElement('span');
        text.className = 'min-w-0';
        const node = document.createElement('span');
        node.className = 'block break-all font-medium';
        node.textContent = permission.node;
        const description = document.createElement('span');
        description.className = 'block text-xs text-gray-500 dark:text-gray-400';
        description.textContent = permission.description;
        text.append(node, description);
        if (permission.value_type) {
            const type = document.createElement('span');
            type.className = `${ADMIN_CHIP_CLASS} mt-2 inline-block`;
            type.textContent = `${permission.value_type}, merge ${permission.merge}`;
            text.appendChild(type);
        }
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = ADMIN_BUTTON_CLASS;
        remove.textContent = 'Delete';
        remove.onclick = () => deleteAdminPermission(permission, remove);
        item.append(text, remove);
        list.appendChild(item);
    });
    document.getElementById('admin-permissions-empty').hidden = permissions.length > 0;
}

function syncAdminPermissionMerge() {
    document.getElementById('admin-permission-create-merge-field').hidden = document.getElementById('admin-permission-create-type').value !== 'int';
}

function createAdminPermission(event) {
    event.preventDefault();
    const body = {
        node: document.getElementById('admin-permission-create-node').value.trim(),
        description: document.getElementById('admin-permission-create-description').value.trim(),
    };
    const type = document.getElementById('admin-permission-create-type').value;
    if (type) {
        body.value_type = type;
    }
    if (type === 'int') {
        body.merge = document.getElementById('admin-permission-create-merge').value;
    }
    runAdminAction(
        document.getElementById('admin-permission-create-submit'),
        () =>
            adminJSON('POST', '/permissions', body).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to create the permission');
                }
                document.getElementById('admin-permission-create-form').reset();
                syncAdminPermissionMerge();
                return loadAdminPermissions();
            }),
        'Failed to create the permission'
    );
}

function deleteAdminPermission(permission, button) {
    if (!window.confirm(`Delete the permission ${permission.node}?`)) {
        return;
    }
    runAdminAction(
        button,
        () =>
            adminRequest(`/permissions/${encodeURIComponent(permission.id)}`, { method: 'DELETE' }).then((res) => {
                if (!res.ok) {
                    return showAdminProblem(res, 'Failed to delete the permission');
                }
                return loadAdminPermissions();
            }),
        'Failed to delete the permission'
    );
}
