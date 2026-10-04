import { test, expect, signIn, STUB, APP, error } from './admin-helpers.js';

test.describe('admin - access', () => {
  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/users');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('a session the API stops accepting sends an action to the login page', async ({ page }) => {
    await signIn(page);
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/permissions');
    await expect(page.locator('#admin-permissions li').first()).toBeVisible();
    await page.context().clearCookies();
    page.once('dialog', (dialog) => dialog.accept());
    await page.locator('#admin-permissions li').first().getByRole('button', { name: 'Delete' }).click();
    await expect(page).toHaveURL(/\/login$/);
  });

  test('admin actions refuse a request that is not from htmx', async ({ page }) => {
    await signIn(page);
    const res = await page.request.post(`${APP}/admin/roles`, { form: { name: 'sneaky' } });
    expect(res.status()).toBe(403);
  });

  test('a server that cannot be reached shows a message in the banner', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await page.route('**/admin/permissions/*', (route) => route.abort('failed'));
    page.once('dialog', (dialog) => dialog.accept());
    await page.locator('#admin-permissions li').first().getByRole('button', { name: 'Delete' }).click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
  });

  test('going back after the session ends does not show the page again', async ({ page }) => {
    await signIn(page);
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-roles li').first()).toBeVisible();
    await page.goto('/admin');
    await page.context().clearCookies();
    await page.goBack();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('admin - settings link', () => {
  async function openAccount(page, permissions) {
    const json = (body) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    await signIn(page, { me: permissions });
    await page.route(`${STUB}/api/v1/users/me`, (route) => route.fulfill(json({ username: 'admin' })));
    await page.route(`${STUB}/api/v1/users/me/links`, (route) => route.fulfill(json([])));
    await page.route(`${STUB}/api/v1/users/me/settings`, (route) => route.fulfill(json({ password_auth: true })));
    const answered = page.waitForResponse('**/account/admin-link');
    await page.goto('/account');
    await answered;
  }

  for (const permissions of [['users.admin'], ['roles.admin'], ['ratelimit:1000', 'users.admin'], ['roles.admin:1']]) {
    test(`the account page links to the admin dashboard with ${permissions.join(', ')}`, async ({ page }) => {
      await openAccount(page, permissions);
      await expect(page.locator('#admin-dashboard-link')).toBeVisible();
      await expect(page.locator('#admin-dashboard-link')).toHaveAttribute('href', '/admin');
    });
  }

  for (const permissions of [[], ['ratelimit:1000'], ['beenamegenerator.admin'], ['users.administrator'], ['xusers.admin'], ['roles.adminx:1']]) {
    test(`the account page has no admin link with [${permissions.join(', ')}]`, async ({ page }) => {
      await openAccount(page, permissions);
      await expect(page.locator('#account-username')).toHaveText('admin');
      await expect(page.locator('#admin-dashboard-link')).toHaveCount(0);
      await expect(page.locator('[hx-get="/account/admin-link"]')).toHaveCount(0);
    });
  }

  test('the account page has no admin link when the session is not accepted', async ({ page }) => {
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    const answered = page.waitForResponse('**/account/admin-link');
    await page.goto('/account');
    await answered;
    await expect(page.locator('#admin-dashboard-link')).toHaveCount(0);
  });
});

test.describe('bee name generator - admin link', () => {
  test('the page links to the suggestion review with the bee admin permission', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto('/project/bee-name-generator');
    await expect(page.locator('#bee-admin-link')).toHaveAttribute('href', '/project/bee-name-generator/admin');
  });

  for (const permissions of [[], ['users.admin', 'roles.admin'], ['beenamegenerator|*']]) {
    test(`the page has no review link with [${permissions.join(', ')}]`, async ({ page }) => {
      await signIn(page, { me: permissions });
      const answered = page.waitForResponse('**/project/bee-name-generator/admin-link');
      await page.goto('/project/bee-name-generator');
      await answered;
      await expect(page.locator('#bee-admin-link')).toHaveCount(0);
      await expect(page.locator('[hx-get="/project/bee-name-generator/admin-link"]')).toHaveCount(0);
    });
  }
});

test.describe('admin - dashboard', () => {
  test('shows the cards for the permissions held', async ({ page }) => {
    await signIn(page, { me: ['users.admin'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-users-link')).toHaveAttribute('href', '/admin/users');
    await expect(page.locator('#admin-roles-link')).toHaveCount(0);
    await expect(page.locator('#admin-permissions-link')).toHaveCount(0);
    await expect(page.locator('#admin-denied')).toHaveCount(0);

    await signIn(page, { me: ['roles.admin'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-roles-link')).toHaveAttribute('href', '/admin/roles');
    await expect(page.locator('#admin-permissions-link')).toHaveAttribute('href', '/admin/permissions');
    await expect(page.locator('#admin-users-link')).toHaveCount(0);
  });

  test('an account without admin permissions sees that it has none', async ({ page }) => {
    await signIn(page, { me: ['ratelimit:1000'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-denied')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toHaveCount(0);
  });

  test('a failed permissions lookup shows the API message', async ({ page }) => {
    await signIn(page, { failures: { 'GET /users/me/permissions': { status: 500, detail: 'no access' } } });
    const res = await page.goto('/admin');
    expect(res.status()).toBe(500);
    await expect(error(page)).toHaveText('no access');
    await expect(page.locator('#admin-denied')).toHaveCount(0);
  });
});
