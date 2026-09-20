/**
 * @description This function is used to set a cookie
 * @param name {string} - The name of the cookie to set
 * @param value {string} - The value of the cookie
 * @param expires {string} - The expiration date of the cookie
 */
function setCookie(name, value, expires) {
    document.cookie = name + "=" + value + "; expires=" + expires + "; path=/; domain=.neuralnexus.dev; SameSite=None; Secure=true";
}

/**
 * @description A session object
 * @typedef {Object} Session
 * @property {string} session - The session JWT
 */

/**
 * @description This function is used to get the user ID from the cookie
 * @param data {Session} - The data object containing the user ID
 */
function updateSession(data) {
    if (data.session) {
        const payload = JSON.parse(atob(data.session.split('.')[1]));
        if (payload && payload.exp) {
            const exp = payload.exp * 1000;
            setCookie('session', payload.session, new Date(exp).toUTCString());
        }
    }
}

function logout() {
    fetch('https://api.neuralnexus.dev/api/v1/auth/logout', {
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
    fetch('https://api.neuralnexus.dev/api/v1/auth/login', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(json)
    })
        .then((res) => res.json())
        .then((data) => updateSession(data))
        .catch((error) => {
            console.error('Error:', error)
        })
}

/**
 * @description Generate a random nonce for the OAuth flow
 * @returns {string} - The generated nonce
 */
function generateNonce() {
    const nonce = Math.random().toString(36).substring(2, 15);
    setCookie('nonce', nonce, new Date(Date.now() + 60 * 1000).toUTCString());
    return nonce;
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
 * @description Loads the profile into the account page via /me, since the
 * session cookie resolves identity server-side - no user ID needed here.
 */
function loadAccountProfile() {
    fetch('https://api.neuralnexus.dev/api/v1/users/me', {
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
const LINK_PLATFORMS = ['discord', 'twitch', 'microsoft', 'xboxlive', 'minecraft'];

/**
 * @description Tracks the most recent loadLinkedAccounts request. It's
 * triggered from several places (initial load, a successful unlink, a
 * successful login-enabled toggle) whose responses can arrive out of
 * order; only the response to the most recently issued request is
 * applied, so a stale refresh can't repaint the rows over a newer one.
 */
let loadLinkedAccountsSeq = 0;

/**
 * @description Fetches the caller's linked accounts and updates each
 * platform row's verified/login-enabled/unlink state, redirecting to
 * /login if the session is missing or expired.
 */
function loadLinkedAccounts() {
    const seq = ++loadLinkedAccountsSeq;

    fetch('https://api.neuralnexus.dev/api/v1/users/me/links', {
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

    const base = document.getElementById(LINK_OAUTH_BASE_IDS[platform]);
    if (!base) {
        return;
    }
    const state = {
        platform: platform,
        nonce: linkNonce,
        redirect_uri: linkRedirect,
        mode: 'link'
    };
    window.location.href = base.innerText + '&state=' + encodeState(state);
}

/**
 * @description Tracks the most recent link/unlink or login-toggle request
 * issued per platform. Both unlinkPlatform and setPlatformLoginEnabled
 * bump this before firing their request and check it in their failure
 * handler, so a slow/out-of-order failure from a request that's since
 * been superseded - by unlinking the platform, or by another toggle -
 * doesn't misleadingly revert the checkbox or pop an alert for an action
 * the user has already moved past. setPlatformLoginEnabled's success
 * handler checks it too, so a stale toggle's now-pointless success
 * doesn't trigger an extra refresh once a newer action for the same
 * platform has taken over; unlinkPlatform's own success always refreshes
 * unconditionally, since an unlink is never itself superseded by a
 * later-issued toggle response - its own DELETE result is authoritative.
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

    fetch(`https://api.neuralnexus.dev/api/v1/users/me/link/${platform}`, {
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

    fetch(`https://api.neuralnexus.dev/api/v1/users/me/link/${platform}`, {
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
