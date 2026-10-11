import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;
const SITE = process.env.BASE_URL || 'http://localhost:8099';
const STEAM = process.env.STEAM_OPENID_LOGIN_URL;
const SCRIPTS = fileURLToPath(new URL('../../../public/js/scripts.js', import.meta.url));

const encodeProblem = (problem) =>
  Buffer.from(JSON.stringify(problem), 'utf8').toString('base64').replace(/\+/g, '-').replace(/\//g, '_');

const decodeState = (state) => JSON.parse(Buffer.from(state.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8'));

const nonceCookie = async (context) => (await context.cookies()).find((cookie) => cookie.name === 'nonce');

async function stubProviders(page) {
  const hosts = ['discord.com', 'id.twitch.tv', 'login.microsoftonline.com', new URL(STEAM).hostname];
  await page.route((url) => hosts.includes(url.hostname), (route) => route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' }));
}

/** Serves a bare page with the two hidden elements and the real scripts.js on a host the stub does not know, so cookies can be shared across subdomains. */
async function fakeSite(page, origin, apiUrl) {
  await page.route(`${origin}/**`, (route) => {
    if (new URL(route.request().url()).pathname === '/scripts.js') {
      return route.fulfill({ path: SCRIPTS });
    }
    return route.fulfill({
      contentType: 'text/html',
      body: `<!doctype html><html><body><div hidden id="api-base-url">${apiUrl}</div><div hidden id="steam-openid-login-url">${STEAM}</div><script src="/scripts.js"></script></body></html>`,
    });
  });
  await page.goto(`${origin}/`);
}

test.describe('apiBaseUrl', () => {
  test('SC-01 reads the API origin from the hidden element the server publishes', async ({ page }) => {
    await page.goto('/');
    expect(await page.evaluate(() => apiBaseUrl())).toBe(process.env.NN_API_URL);
  });
});

test.describe('showAuthErrorFromQuery', () => {
  const problem = (detail) => encodeProblem({ type: 'about:blank', status: 400, title: 'Bad Request', detail });

  test('SC-02 a problem in the query shows its detail in the banner and leaves the URL without it', async ({ page }) => {
    await page.goto(`/login?problem=${problem('Invalid state ???>>>')}`);

    await expect(page.locator('#auth-error')).toBeVisible();
    await expect(page.locator('#auth-error')).toHaveText('Invalid state ???>>>');
    expect(new URL(page.url()).searchParams.has('problem')).toBe(false);
  });

  test('SC-03 only the problem is removed from the URL, the other parameters and the hash stay', async ({ page }) => {
    await page.goto(`/login?next=/account&problem=${problem('Authentication failed')}#top`);

    await expect(page.locator('#auth-error')).toHaveText('Authentication failed');
    const url = new URL(page.url());
    expect(url.searchParams.has('problem')).toBe(false);
    expect(url.searchParams.get('next')).toBe('/account');
    expect(url.hash).toBe('#top');
  });

  test('SC-04 the URL is replaced and not added to the history', async ({ page }) => {
    await page.goto('/');
    await page.goto(`/login?problem=${problem('Invalid state')}`);
    await expect(page.locator('#auth-error')).toBeVisible();
    await page.goBack();
    expect(new URL(page.url()).pathname).toBe('/');
  });

  test('SC-05 without a problem the banner stays hidden and the URL is untouched', async ({ page }) => {
    await page.goto('/login?next=/account');
    await expect(page.locator('#auth-error')).toBeHidden();
    expect(new URL(page.url()).search).toBe('?next=/account');
  });

  test('SC-06 a problem without a detail shows the generic message', async ({ page }) => {
    await page.goto(`/login?problem=${encodeProblem({ status: 500, title: 'Internal Server Error' })}`);
    await expect(page.locator('#auth-error')).toHaveText('Something went wrong. Please try again.');
  });

  test('SC-07 a problem that cannot be decoded leaves the banner hidden and is logged', async ({ page }) => {
    const logged = [];
    page.on('console', (message) => message.type() === 'error' && logged.push(message.text()));
    await page.goto('/login?problem=not*base64');
    await expect(page.locator('#auth-error')).toBeHidden();
    expect(new URL(page.url()).searchParams.has('problem')).toBe(false);
    expect(logged.some((text) => text.startsWith('Error:'))).toBe(true);
  });

  test('SC-08 the detail is shown as text, never as HTML', async ({ page }) => {
    await page.goto(`/login?problem=${problem('<img src=x onerror="window.__xss=1">')}`);
    await expect(page.locator('#auth-error')).toHaveText('<img src=x onerror="window.__xss=1">');
    await expect(page.locator('#auth-error img')).toHaveCount(0);
    expect(await page.evaluate(() => window.__xss)).toBeUndefined();
  });

  test('SC-09 on a page without the banner the problem is still removed from the URL', async ({ page }) => {
    await page.goto(`/projects?problem=${problem('Invalid state')}`);
    await page.evaluate(() => showAuthErrorFromQuery());
    expect(new URL(page.url()).search).toBe('');
  });
});

test.describe('steamOpenIdLoginUrl', () => {
  test('SC-10 reads the Steam login address from the hidden element the server publishes', async ({ page }) => {
    await page.goto('/');
    expect(await page.evaluate(() => steamOpenIdLoginUrl())).toBe(STEAM);
  });
});

test.describe('sharedHostSuffix', () => {
  const cases = [
    ['neuralnexus.dev', 'api.neuralnexus.dev', '.neuralnexus.dev', 'a bare domain against its subdomain'],
    ['localhost', 'localhost', '', 'the same host'],
    ['localhost', 'api.neuralnexus.dev', '', 'nothing shared'],
    ['neuralnexus.dev', 'other.dev', '', 'one label shared'],
    ['10.0.0.1', '192.168.0.1', '', 'IPv4 addresses'],
    ['::1', 'api.example.com', '', 'an IPv6 address'],
    ['a.co.uk', 'b.co.uk', '.co.uk', 'exactly two labels shared'],
    ['www.example.co.uk', 'api.example.co.uk', '.example.co.uk', 'three labels shared'],
  ];
  for (const [site, api, expected, name] of cases) {
    test(`SC-11 ${site} + ${api} -> "${expected}" (${name})`, async ({ page }) => {
      await page.goto('/');
      expect(await page.evaluate(([s, a]) => sharedHostSuffix(s, a), [site, api])).toBe(expected);
    });
  }
});

test.describe('setNonceCookie', () => {
  test('SC-12 without a domain it sets a host-only cookie that lasts five minutes', async ({ page, context }) => {
    await page.goto('/');
    await page.evaluate(() => setNonceCookie('abc123', ''));
    const cookie = await nonceCookie(context);
    expect(cookie).toMatchObject({ value: 'abc123', domain: 'localhost', path: '/', sameSite: 'Lax', secure: false });
    const remaining = cookie.expires - Date.now() / 1000;
    expect(remaining).toBeGreaterThan(290);
    expect(remaining).toBeLessThanOrEqual(300);
  });

  test('SC-13 a rejected Domain falls back to a host-only cookie', async ({ page, context }) => {
    await page.goto('/');
    await page.evaluate(() => setNonceCookie('abc123', '.co.uk'));
    const cookie = await nonceCookie(context);
    expect(cookie).toMatchObject({ value: 'abc123', domain: 'localhost' });
  });

  test('SC-14 an accepted Domain makes the cookie reach the sibling subdomain', async ({ page, context }) => {
    await fakeSite(page, 'http://app.example.test', 'http://api.example.test:8098');
    await page.evaluate(() => setNonceCookie('abc123', '.example.test'));
    const forApi = await context.cookies('http://api.example.test/');
    expect(forApi.find((cookie) => cookie.name === 'nonce')).toMatchObject({ value: 'abc123', domain: '.example.test' });
  });

  test('SC-15 on https the cookie is SameSite=None and Secure', async ({ page, context }) => {
    await fakeSite(page, 'https://app.example.test', 'https://api.example.test');
    await page.evaluate(() => setNonceCookie('abc123', '.example.test'));
    const cookie = (await context.cookies('https://api.example.test/')).find((c) => c.name === 'nonce');
    expect(cookie).toMatchObject({ value: 'abc123', sameSite: 'None', secure: true });
  });
});

test.describe('createNonce', () => {
  test('SC-16 it returns a short alphanumeric nonce, stores it, and a second call replaces it', async ({ page, context }) => {
    await page.goto('/');
    const first = await page.evaluate(() => createNonce());
    expect(first).toMatch(/^[a-z0-9]{1,13}$/);
    expect((await nonceCookie(context)).value).toBe(first);
    const second = await page.evaluate(() => createNonce());
    expect(second).not.toBe(first);
    expect((await nonceCookie(context)).value).toBe(second);
  });

  test('SC-17 on a site that shares a domain with the API the cookie reaches the API host', async ({ page, context }) => {
    await fakeSite(page, 'http://app.example.test', 'http://api.example.test:8098');
    const nonce = await page.evaluate(() => createNonce());
    const forApi = await context.cookies('http://api.example.test/');
    expect(forApi.find((cookie) => cookie.name === 'nonce')).toMatchObject({ value: nonce, domain: '.example.test' });
  });
});

test.describe('checkHeaderAuthState', () => {
  const controls = (page) => ({
    account: page.locator('#header-account'),
    username: page.locator('#header-account-username'),
    logout: page.locator('#header-logout-btn'),
    login: page.locator('#header-login-btn'),
  });

  test('SC-18 a signed-in visitor sees the account with the username and the logout button, not the login button', async ({ page }) => {
    await page.route(`${API}/users/me`, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ username: 'gearhead' }) })
    );
    await page.goto('/');
    const { account, username, logout, login } = controls(page);
    await expect(account).toBeVisible();
    await expect(username).toHaveText('gearhead');
    await expect(account.getByRole('link', { name: 'Account settings' })).toHaveAttribute('href', '/account');
    await expect(logout).toBeVisible();
    await expect(login).toBeHidden();
  });

  test('SC-19 a signed-out visitor sees the login button, not the account or the logout button', async ({ page }) => {
    await page.route(`${API}/users/me`, (route) => route.fulfill({ status: 401 }));
    await page.goto('/');
    const { account, logout, login } = controls(page);
    await expect(login).toBeVisible();
    await expect(account).toBeHidden();
    await expect(logout).toBeHidden();
  });

  test('SC-20 the request carries the session cookie', async ({ page, context }) => {
    await context.addCookies([{ name: 'session', value: 'abc123', url: SITE }]);
    let cookie = null;
    await page.route(`${API}/users/me`, async (route) => {
      cookie = (await route.request().allHeaders()).cookie;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ username: 'gearhead' }) });
    });
    await page.goto('/');
    await expect(controls(page).account).toBeVisible();
    expect(cookie).toContain('session=abc123');
  });

  test('SC-21 an API that cannot be reached shows none of the controls and logs the error', async ({ page }) => {
    const logged = [];
    page.on('console', (message) => message.type() === 'error' && logged.push(message.text()));
    await page.route(`${API}/users/me`, (route) => route.abort('failed'));
    await page.goto('/');
    await expect.poll(() => logged.some((text) => text.startsWith('Error:'))).toBe(true);
    const { account, logout, login } = controls(page);
    await expect(account).toBeHidden();
    await expect(logout).toBeHidden();
    await expect(login).toBeHidden();
  });
});

test.describe('logout', () => {
  const signedIn = (page) =>
    page.route(`${API}/users/me`, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ username: 'gearhead' }) })
    );

  test('SC-22 a 204 sends a POST with the session cookie and goes to the home page', async ({ page, context }) => {
    await context.addCookies([{ name: 'session', value: 'abc123', url: SITE }]);
    await signedIn(page);
    let request = null;
    await page.route(`${API}/auth/logout`, async (route) => {
      request = { method: route.request().method(), cookie: (await route.request().allHeaders()).cookie };
      await route.fulfill({ status: 204 });
    });
    await page.goto('/login');
    await page.locator('#header-logout-btn button').click();
    await page.waitForURL((url) => url.pathname === '/');
    expect(request.method).toBe('POST');
    expect(request.cookie).toContain('session=abc123');
  });

  test('SC-23 a failed logout stays on the page and logs it', async ({ page }) => {
    await signedIn(page);
    await page.route(`${API}/auth/logout`, (route) =>
      route.fulfill({ status: 500, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'boom' }) })
    );
    await page.goto('/login');
    const logged = page.waitForEvent('console', (message) => message.text() === 'Logout failed');
    await page.locator('#header-logout-btn button').click();
    await logged;
    expect(new URL(page.url()).pathname).toBe('/login');
  });

  test('SC-24 a logout that cannot reach the API stays on the page and logs the error', async ({ page }) => {
    await signedIn(page);
    await page.route(`${API}/auth/logout`, (route) => route.abort('failed'));
    await page.goto('/login');
    const logged = page.waitForEvent('console', (message) => message.type() === 'error' && message.text().startsWith('Error:'));
    await page.locator('#header-logout-btn button').click();
    await logged;
    expect(new URL(page.url()).pathname).toBe('/login');
  });
});

test.describe('submitLoginForm', () => {
  const submit = async (page, username, password) => {
    await page.goto('/login');
    await page.fill('#username', username);
    await page.fill('#password', password);
    await page.click('button[type="submit"]');
  };

  test('SC-25 a username is sent as JSON with the password and the visitor goes to the home page', async ({ page }) => {
    let request = null;
    await page.route(`${API}/auth/login`, (route) => {
      request = { body: route.request().postDataJSON(), type: route.request().headers()['content-type'] };
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    });
    await submit(page, 'someone', 'correct-password');
    await page.waitForURL((url) => url.pathname === '/');
    expect(request.body).toEqual({ password: 'correct-password', username: 'someone' });
    expect(request.type).toBe('application/json');
  });

  test('SC-26 an address with an @ is sent as the email and not as the username', async ({ page }) => {
    let body = null;
    await page.route(`${API}/auth/login`, (route) => {
      body = route.request().postDataJSON();
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    });
    await submit(page, 'someone@example.com', 'hunter2');
    await page.waitForURL((url) => url.pathname === '/');
    expect(body).toEqual({ password: 'hunter2', email: 'someone@example.com' });
  });

  test('SC-27 a refused login shows the detail in an alert and stays on the login page', async ({ page }) => {
    await page.route(`${API}/auth/login`, (route) =>
      route.fulfill({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'Invalid username or password' }) })
    );
    const dialog = page.waitForEvent('dialog');
    await submit(page, 'someone', 'wrong-password');
    const alert = await dialog;
    expect(alert.message()).toBe('Invalid username or password');
    await alert.dismiss();
    expect(page.url()).toBe(`${SITE}/login`);
  });

  test('SC-28 a login that cannot reach the API shows an alert and stays on the login page', async ({ page }) => {
    await page.route(`${API}/auth/login`, (route) => route.abort('failed'));
    const dialog = page.waitForEvent('dialog');
    await submit(page, 'someone', 'correct-password');
    const alert = await dialog;
    expect(alert.message()).toBe('Failed to fetch');
    await alert.dismiss();
    expect(page.url()).toBe(`${SITE}/login`);
  });
});

test.describe('encodeState', () => {
  test('SC-29 the state is base64url with padding, and decodes back to the state', async ({ page }) => {
    await page.goto('/');
    const states = [
      { q: '???>>>' },
      { platform: 'discord', nonce: 'abc', redirect_uri: `${SITE}/login?next=/some/path?a=1&b=2`, mode: 'login' },
    ];
    for (const state of states) {
      const { encoded, plain } = await page.evaluate((s) => ({ encoded: encodeState(s), plain: btoa(JSON.stringify(s)) }), state);
      expect(plain, 'the state must need the url-safe alphabet').toMatch(/[+/]/);
      expect(encoded).not.toMatch(/[+/]/);
      expect(encoded.length % 4).toBe(0);
      expect(decodeState(encoded)).toEqual(state);
    }
  });
});

test.describe('buildSteamOpenIDURL', () => {
  test('SC-30 it builds an OpenID 2.0 request to the Steam address with the state in the return address', async ({ page }) => {
    await page.goto('/');
    const state = { platform: 'steam', nonce: 'abc', redirect_uri: SITE, mode: 'link' };
    const built = await page.evaluate((s) => buildSteamOpenIDURL(s), state);

    const url = new URL(built);
    expect(url.origin + url.pathname).toBe(STEAM);
    expect(url.searchParams.get('openid.ns')).toBe('http://specs.openid.net/auth/2.0');
    expect(url.searchParams.get('openid.mode')).toBe('checkid_setup');
    expect(url.searchParams.get('openid.realm')).toBe(`${process.env.NN_API_URL}/`);
    expect(url.searchParams.get('openid.identity')).toBe('http://specs.openid.net/auth/2.0/identifier_select');
    expect(url.searchParams.get('openid.claimed_id')).toBe('http://specs.openid.net/auth/2.0/identifier_select');
    expect(url.searchParams.has('state')).toBe(false);
    const returnTo = new URL(url.searchParams.get('openid.return_to'));
    expect(returnTo.origin + returnTo.pathname).toBe(`${process.env.NN_API_URL}/api/openid`);
    expect(decodeState(returnTo.searchParams.get('state'))).toEqual(state);
  });
});

test.describe('startOAuthLogin', () => {
  const providers = [
    ['discord', 'discord-oauth'],
    ['twitch', 'twitch-oauth'],
    ['xboxlive', 'xbox-oauth'],
    ['microsoft', 'microsoft-oauth'],
  ];

  for (const [platform, button] of providers) {
    test(`SC-31 the ${platform} button goes to the provider address with the login state`, async ({ page, context }) => {
      await stubProviders(page);
      await page.goto('/login');
      const base = await page.locator(`#${platform}-oauth-base`).evaluate((element) => element.innerText);
      await page.locator(`#${button}`).click();
      await page.waitForURL((url) => !url.href.startsWith(SITE));

      expect(page.url().startsWith(`${base}&state=`)).toBe(true);
      const state = decodeState(new URL(page.url()).searchParams.get('state'));
      expect(state).toMatchObject({ platform, mode: 'login', redirect_uri: SITE });
      expect(state.nonce).toBe((await nonceCookie(context)).value);
    });
  }

  test('SC-32 the Steam button goes to the OpenID request with the login state in the return address', async ({ page, context }) => {
    await stubProviders(page);
    await page.goto('/login');
    await page.locator('#steam-oauth').click();
    await page.waitForURL((url) => url.href.startsWith(STEAM));

    const returnTo = new URL(new URL(page.url()).searchParams.get('openid.return_to'));
    const state = decodeState(returnTo.searchParams.get('state'));
    expect(state).toMatchObject({ platform: 'steam', mode: 'login', redirect_uri: SITE });
    expect(state.nonce).toBe((await nonceCookie(context)).value);
  });

  for (const [name, path, redirect] of [
    ['/login', '/login', SITE],
    ['/register', '/register', SITE],
    ['/login with a query string', '/login?next=/some/path?a=1&b=2', `${SITE}/login?next=/some/path?a=1&b=2`],
  ]) {
    test(`SC-33 from ${name} the return address is ${redirect}`, async ({ page }) => {
      await stubProviders(page);
      await page.goto(path);
      await page.locator('#discord-oauth').click();
      await page.waitForURL((url) => url.hostname === 'discord.com');
      expect(decodeState(new URL(page.url()).searchParams.get('state')).redirect_uri).toBe(redirect);
    });
  }

  test('SC-34 the nonce cookie is not set until a login button is clicked', async ({ page, context }) => {
    await stubProviders(page);
    await page.goto('/login');
    expect(await nonceCookie(context)).toBeUndefined();
    await page.locator('#discord-oauth').click();
    await page.waitForURL((url) => url.hostname === 'discord.com');
    expect(await nonceCookie(context)).toBeTruthy();
  });
});
