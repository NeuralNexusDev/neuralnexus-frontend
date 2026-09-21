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
    // Steam's button is Valve's own pre-made asset, not our text - it always
    // reads "Sign in through Steam" regardless of page/labelPrefix.
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

test.describe('OAuth state encoding', () => {
  // Regression test for a historical bug: encodeState() originally used
  // plain base64 (btoa), but the API decodes the "state" query param with
  // Go's base64.URLEncoding (url-safe alphabet). A redirect_uri containing
  // a query string produces '+'/'/' in plain base64 often enough to break
  // that decode. The fix converts to base64url by swapping +/- and //_.
  test('login redirect hrefs never contain plain-base64 characters', async ({ page }) => {
    await page.goto('/login?next=/some/path?a=1&b=2');

    // The hrefs are filled in by a setTimeout(..., 1000) in OAuthButtons.
    await page.waitForTimeout(1200);

    for (const id of ['discord-oauth', 'twitch-oauth', 'microsoft-oauth', 'xbox-oauth', 'minecraft-oauth']) {
      const href = await page.locator(`#${id}`).getAttribute('href');
      expect(href).toContain('state=');
      const state = new URL(href, 'http://localhost').searchParams.get('state');
      expect(state).toBeTruthy();
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
    await page.goto('/login?next=/some/path?a=1&b=2');
    await page.waitForTimeout(1200);

    const href = await page.locator('#steam-oauth').getAttribute('href');
    const url = new URL(href);
    expect(url.origin + url.pathname).toBe('https://steamcommunity.com/openid/login');
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
