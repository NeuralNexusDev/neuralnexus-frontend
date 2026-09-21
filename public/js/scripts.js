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
 * @description Generates a fresh nonce for the OAuth/OpenID flow and sets
 * it as a short-lived cookie the API checks on the callback, returning the
 * nonce. Called at the moment the user clicks a login/link button (not on
 * page load) so its 5-minute TTL covers the provider round-trip rather than
 * however long the user sat on the page first. The hardcoded production
 * domain/Secure/SameSite=None only apply on neuralnexus.dev itself - a
 * browser rejects a Domain attribute that doesn't match the current host,
 * so on localhost (or any other dev/test host) this falls back to a
 * host-only cookie with SameSite=Lax and no Secure flag.
 * @returns {string} - The generated nonce
 */
function createNonce() {
    const nonce = Math.random().toString(36).substring(2, 15);

    const host = location.hostname;
    const isProdDomain = host === 'neuralnexus.dev' || host.endsWith('.neuralnexus.dev');
    const domainAttr = isProdDomain ? '; domain=.neuralnexus.dev' : '';
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

    loadLinkedAccounts();
}

/**
 * @description The platforms shown as rows on the account settings page.
 */
const LINK_PLATFORMS = ['discord', 'twitch', 'microsoft', 'xboxlive', 'minecraft', 'steam'];

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
 * Minecraft has no OAuth app of its own - it shares Xbox Live's, and the
 * API tells the two apart by the "platform" field in the OAuth state.
 */
const LINK_OAUTH_BASE_IDS = {
    discord: 'link-discord-oauth-base',
    twitch: 'link-twitch-oauth-base',
    microsoft: 'link-microsoft-oauth-base',
    xboxlive: 'link-xboxlive-oauth-base',
    minecraft: 'link-xboxlive-oauth-base'
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
