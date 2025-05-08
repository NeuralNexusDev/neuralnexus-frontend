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

function setCookie(name, value, expires) {
    document.cookie = name + "=" + value + "; expires=" + expires + "; path=/";
}

function getSession() {
    return getCookie('sessionId');
}

function updateSession(data) {
    if (data.session_id) {
        setCookie('session_id', data.session_id, new Date(data.exp).toUTCString());
        setCookie('user_id', data.user_id, new Date(data.exp).toUTCString());
        window.location.href = '/';
    }
}

function revokeSession() {
    fetch('https://api.neuralnexus.dev/api/v1/auth/logout', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + getSession()
        }
    })
        .then((res) => res.json())
        .then((data) => {
            if (data.success) {
                setCookie('session_id', '', new Date(0).toUTCString());
                setCookie('user_id', '', new Date(0).toUTCString());
                window.location.href = '/'
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
