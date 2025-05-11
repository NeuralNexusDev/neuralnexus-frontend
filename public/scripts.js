/**
 * @description This function is used to get a cookie by name
 * @param cname {string} - The name of the cookie to get
 * @returns {string} - The value of the cookie
 */
function getCookie(cname) {
    let name = cname + "=";
    let decodedCookie = decodeURIComponent(document.cookie);
    let ca = decodedCookie.split(';');
    for(let i = 0; i <ca.length; i++) {
        let c = ca[i];
        while (c.charAt(0) === ' ') {
            c = c.substring(1);
        }
        if (c.indexOf(name) === 0) {
            return c.substring(name.length, c.length);
        }
    }
    return "";
}

/**
 * @description This function is used to set a cookie
 * @param name {string} - The name of the cookie to set
 * @param value {string} - The value of the cookie
 * @param expires {string} - The expiration date of the cookie
 */
function setCookie(name, value, expires) {
    document.cookie = name + "=" + value + "; expires=" + expires + "; path=/";
}

/**
 * @description This function is used to delete a cookie
 * @param name {string} - The name of the cookie to delete
 */
function deleteCookie(name) {
    document.cookie = name + '=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
}

/**
 * @description This function is used to get the session ID from the cookie
 * @returns {string} - The session ID
 */
function getSession() {
    return getCookie('session');
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
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + getSession()
        }
    })
        .then((res) => {
            if (res.status === 204) {
                deleteCookie('session');
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

function generateNonce() {
    const nonce = Math.random().toString(36).substring(2, 15);
    setCookie('nonce', nonce, new Date(Date.now() + 60 * 1000).toUTCString());
    return nonce;
}

function validateNonce(nonce) {
    const cookieNonce = getCookie('nonce');
    if (cookieNonce === nonce) {
        setCookie('nonce', '', new Date(0).toUTCString());
        return true;
    }
    return false;
}

function encodeState(platform, nonce, redirect_uri) {
    const state = {
        platform: platform,
        nonce: nonce,
        redirect_uri: redirect_uri
    };
    return btoa(JSON.stringify(state));
}

function decodeState(state) {
    const decoded = atob(state);
    return JSON.parse(decoded);
}
