import { test, expect, signIn, APP, error } from './helpers.js';

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
    await signIn(page, { me: permissions, account: { username: 'admin', password_auth: true } });
    const answered = page.waitForResponse('**/account/admin-link');
    await page.goto('/account');
    await answered;
  }

  test('the account page links to the admin dashboard with an admin permission', async ({ page }) => {
    await openAccount(page, ['users.admin']);
    await expect(page.locator('#admin-dashboard-link')).toHaveAttribute('href', '/admin');
  });

  test('the account page has no admin link without an admin permission', async ({ page }) => {
    await openAccount(page, ['ratelimit:1000']);
    await expect(page.locator('#account-username')).toHaveText('admin');
    await expect(page.locator('#admin-dashboard-link')).toHaveCount(0);
    await expect(page.locator('[hx-get="/account/admin-link"]')).toHaveCount(0);
  });
});

test.describe('bee name generator - admin link', () => {
  test('the page links to the suggestion review with the bee admin permission', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto('/project/bee-name-generator');
    await expect(page.locator('#bee-admin-link')).toHaveAttribute('href', '/project/bee-name-generator/admin');
  });

  test('the page has no review link without the bee admin permission', async ({ page }) => {
    await signIn(page, { me: ['users.admin', 'roles.admin'] });
    const answered = page.waitForResponse('**/project/bee-name-generator/admin-link');
    await page.goto('/project/bee-name-generator');
    await answered;
    await expect(page.locator('#bee-admin-link')).toHaveCount(0);
    await expect(page.locator('[hx-get="/project/bee-name-generator/admin-link"]')).toHaveCount(0);
  });
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

});
