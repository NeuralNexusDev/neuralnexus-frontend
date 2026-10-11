import { test, expect, signIn, error } from './helpers.js';

test.describe('admin - access', () => {
  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/users');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('a server that cannot be reached shows a message in the banner', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await expect(page.locator('#admin-permissions li').first()).toBeVisible();
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
    await expect(page.locator('#admin-users-link')).toBeVisible();
    await page.context().clearCookies();
    await page.goBack();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('bee name generator - admin link', () => {
  test('the page has no review link without the bee admin permission', async ({ page }) => {
    await signIn(page, { me: ['users.admin', 'roles.admin'] });
    const answered = page.waitForResponse('**/project/bee-name-generator/admin-link');
    await page.goto('/project/bee-name-generator');
    await answered;
    await expect(page.locator('[hx-get="/project/bee-name-generator/admin-link"]')).toHaveCount(0);
    await expect(page.locator('#bee-admin-link')).toHaveCount(0);
  });
});
