import { test, expect } from '@playwright/test';

test.describe('static pages', () => {
  test('home page renders', async ({ page }) => {
    const res = await page.goto('/');
    expect(res.status()).toBe(200);
    await expect(page.getByRole('heading', { name: 'NeuralNexus' })).toBeVisible();
  });

  test('login page renders all five OAuth buttons', async ({ page }) => {
    const res = await page.goto('/login');
    expect(res.status()).toBe(200);
    for (const id of ['discord-oauth', 'twitch-oauth', 'microsoft-oauth', 'xbox-oauth', 'minecraft-oauth']) {
      await expect(page.locator(`#${id}`)).toBeAttached();
    }
    await expect(page.getByText('Login with Discord')).toBeVisible();
  });

  test('register page renders all five OAuth buttons with Sign up labels', async ({ page }) => {
    const res = await page.goto('/register');
    expect(res.status()).toBe(200);
    await expect(page.getByText('Sign up with Discord')).toBeVisible();
  });

  // Regression test for the hidden #api-base-url element scripts.js reads
  // instead of hardcoding the backend's origin. Only checks the default
  // (API_BASE_URL unset) - the override path is exercised by pointing the
  // whole suite at a containerized backend via BASE_URL/API_BASE_URL.
  test('exposes the API base URL for scripts.js to read', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#api-base-url')).toHaveText('https://api.neuralnexus.dev');
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
});
