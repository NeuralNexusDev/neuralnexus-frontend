import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;

test.describe('header account section', () => {
  test('stays hidden when logged out', async ({ page }) => {
    await page.route(`${API}/users/me`, (route) => route.fulfill({ status: 401 }));
    await page.goto('/');
    await expect(page.locator('#header-account')).toBeHidden();
  });

  test('shows the username and links to /account when logged in', async ({ page }) => {
    await page.route(`${API}/users/me`, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ username: 'gearhead' }) })
    );
    await page.goto('/');
    await expect(page.locator('#header-account')).toBeVisible();
    await expect(page.locator('#header-account-username')).toHaveText('gearhead');
    await expect(page.locator('#header-account a[aria-label="Account settings"]')).toHaveAttribute('href', '/account');
  });
});
