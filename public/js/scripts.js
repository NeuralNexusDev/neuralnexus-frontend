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

/** Browsers reject a Domain that's a bare TLD or an IP, so those get a host-only cookie. */
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

/** SameSite=None requires Secure, so plain http (local dev) falls back to Lax. */
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

/** State travels inside openid.return_to rather than as an appended query param. */
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

/** Guards against a stale response repainting the toggle. */
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

/** Guards against an out-of-order response repainting the rows. */
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

/** Per-platform guard against a superseded link/unlink/toggle response. */
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

let mcStatusSeq = 0;

/** Server-supplied text goes in via textContent only. */
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

function showMcStatusError(message, detail) {
    document.getElementById('mc-status-result').hidden = true;
    document.getElementById('mc-status-error-message').textContent = message;
    const detailEl = document.getElementById('mc-status-error-detail');
    detailEl.textContent = detail || '';
    detailEl.hidden = !detail;
    document.getElementById('mc-status-error').hidden = false;
}

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
