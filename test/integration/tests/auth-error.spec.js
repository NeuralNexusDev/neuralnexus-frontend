import { test, expect } from '@playwright/test';

// Mirrors nn-api's redirectWithError (modules/auth/routes/auth.go): on an
// OAuth/OpenID failure the API 303-redirects back to the frontend with an
// RFC 9457 problem, base64 (URL-safe, padded - Go's base64.URLEncoding)
// encoded into a "problem" query param, rather than a bare API response the
// browser has no way back from.
function encodeProblem(problem) {
  const json = JSON.stringify(problem);
  return Buffer.from(json, 'utf8').toString('base64').replace(/\+/g, '-').replace(/\//g, '_');
}

test.describe('OAuth/OpenID error redirects', () => {
  test('shows the problem detail on /login and strips the query param', async ({ page }) => {
    const problem = encodeProblem({ type: 'about:blank', status: 400, title: 'Bad Request', detail: 'Invalid state' });
    await page.goto(`/login?problem=${problem}`);

    await expect(page.locator('#auth-error')).toBeVisible();
    await expect(page.locator('#auth-error')).toHaveText('Invalid state');
    expect(new URL(page.url()).searchParams.has('problem')).toBe(false);
  });

  test('shows the problem detail on the home page (pre-validation failures land here)', async ({ page }) => {
    const problem = encodeProblem({ type: 'about:blank', status: 400, title: 'Bad Request', detail: 'Invalid request' });
    await page.goto(`/?problem=${problem}`);

    await expect(page.locator('#auth-error')).toBeVisible();
    await expect(page.locator('#auth-error')).toHaveText('Invalid request');
  });

  test('shows the problem detail on /account (link-mode failures redirect here)', async ({ page }) => {
    await page.route('**/api/v1/users/me', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ username: 'tester' }) })
    );
    await page.route('**/api/v1/users/me/links', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));

    const problem = encodeProblem({ type: 'about:blank', status: 401, title: 'Unauthorized', detail: 'You must be logged in to link an account' });
    await page.goto(`/account?problem=${problem}`);

    await expect(page.locator('#auth-error')).toHaveText('You must be logged in to link an account');
  });

  test('preserves other query params while stripping only "problem"', async ({ page }) => {
    const problem = encodeProblem({ type: 'about:blank', status: 500, title: 'Internal Server Error', detail: 'Authentication failed' });
    await page.goto(`/login?next=/account&problem=${problem}`);

    await expect(page.locator('#auth-error')).toHaveText('Authentication failed');
    const url = new URL(page.url());
    expect(url.searchParams.has('problem')).toBe(false);
    expect(url.searchParams.get('next')).toBe('/account');
  });

  test('no banner shown when there is no problem param', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('#auth-error')).toBeHidden();
  });
});
