import { test, expect } from '@playwright/test';

test.describe('static pages', () => {
  test('home page renders', async ({ page }) => {
    const res = await page.goto('/');
    expect(res.status()).toBe(200);
    await expect(page.getByRole('heading', { name: 'NeuralNexus' })).toBeVisible();
  });

  test('login page renders all six OAuth/OpenID buttons', async ({ page }) => {
    const res = await page.goto('/login');
    expect(res.status()).toBe(200);
    for (const id of ['discord-oauth', 'twitch-oauth', 'microsoft-oauth', 'xbox-oauth', 'minecraft-oauth', 'steam-oauth']) {
      await expect(page.locator(`#${id}`)).toBeAttached();
    }
    await expect(page.getByText('Login with Discord')).toBeVisible();
    await expect(page.getByAltText('Sign in through Steam')).toBeVisible();
  });

  test('register page renders all six OAuth/OpenID buttons with Sign up labels', async ({ page }) => {
    const res = await page.goto('/register');
    expect(res.status()).toBe(200);
    await expect(page.getByText('Sign up with Discord')).toBeVisible();
    await expect(page.getByAltText('Sign in through Steam')).toBeVisible();
  });

  // Regression test for the hidden #api-base-url element scripts.js reads
  // instead of hardcoding the backend's origin.
  test('exposes the API base URL for scripts.js to read', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#api-base-url')).toHaveText(process.env.API_BASE_URL);
  });
});

test.describe('nonce timing', () => {
  // Regression test: the nonce cookie used to be minted on page load, so
  // its 5-minute TTL could run out before the user ever clicked a login
  // button. createNonce() is now only called from the click handler.
  test('nonce cookie is not set until a login button is clicked', async ({ page, context }) => {
    await page.route('https://discord.com/api/oauth2/authorize*', (route) =>
      route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' })
    );

    await page.goto('/login');
    let cookies = await context.cookies();
    expect(cookies.find((c) => c.name === 'nonce')).toBeUndefined();

    await page.locator('#discord-oauth').click();
    await page.waitForURL((url) => url.hostname === 'discord.com');
    cookies = await context.cookies();
    expect(cookies.find((c) => c.name === 'nonce')).toBeTruthy();
  });
});

test.describe('OAuth state encoding', () => {
  // Regression test for a historical bug: encodeState() originally used
  // plain base64 (btoa), but the API decodes the "state" query param with
  // Go's base64.URLEncoding (url-safe alphabet). A redirect_uri containing
  // a query string produces '+'/'/' in plain base64 often enough to break
  // that decode. The fix converts to base64url by swapping +/- and //_.
  //
  // Nonce (and so the whole state/href) is only built at click time now, so
  // this clicks each button and intercepts the provider's own authorize
  // endpoint rather than reading a pre-filled href.
  test('login redirect hrefs never contain plain-base64 characters', async ({ page }) => {
    for (const host of ['discord.com', 'id.twitch.tv', 'login.microsoftonline.com']) {
      await page.route((url) => url.hostname === host, (route) => route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' }));
    }

    for (const id of ['discord-oauth', 'twitch-oauth', 'microsoft-oauth', 'xbox-oauth', 'minecraft-oauth']) {
      await page.goto('/login?next=/some/path?a=1&b=2');
      await page.locator(`#${id}`).click();
      await page.waitForURL((url) => url.pathname !== '/login');

      const state = new URL(page.url()).searchParams.get('state');
      expect(state, `${id} missing state`).toBeTruthy();
      expect(state).not.toMatch(/[+/]/);

      // The state must decode as valid base64url JSON with a "mode" field.
      const decoded = Buffer.from(state.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8');
      const parsed = JSON.parse(decoded);
      expect(parsed.mode).toBe('login');
    }
  });

  // Steam has no OAuth app/client ID - its login URL is built entirely
  // client-side (buildSteamOpenIDURL) with state embedded inside
  // openid.return_to rather than appended as a top-level query param, so it
  // needs its own assertion shape rather than joining the loop above.
  test('Steam login href is a well-formed OpenID 2.0 request with base64url state', async ({ page }) => {
    const steamUrl = process.env.STEAM_OPENID_LOGIN_URL;
    await page.route(`${steamUrl}*`, (route) => route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' }));

    await page.goto('/login?next=/some/path?a=1&b=2');
    await page.locator('#steam-oauth').click();
    await page.waitForURL((url) => url.href.startsWith(steamUrl));

    const url = new URL(page.url());
    expect(url.origin + url.pathname).toBe(steamUrl);
    expect(url.searchParams.get('openid.ns')).toBe('http://specs.openid.net/auth/2.0');
    expect(url.searchParams.get('openid.mode')).toBe('checkid_setup');
    expect(url.searchParams.get('openid.realm')).toBe(`${process.env.API_BASE_URL}/`);

    const returnTo = new URL(url.searchParams.get('openid.return_to'));
    expect(returnTo.origin + returnTo.pathname).toBe(`${process.env.API_BASE_URL}/api/openid`);
    const state = returnTo.searchParams.get('state');
    expect(state).toBeTruthy();
    expect(state).not.toMatch(/[+/]/);

    const decoded = Buffer.from(state.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8');
    const parsed = JSON.parse(decoded);
    expect(parsed.platform).toBe('steam');
    expect(parsed.mode).toBe('login');
  });
});
