import { test, expect } from '@playwright/test';

const API = 'https://api.neuralnexus.dev/api/v1';

test.describe('password login', () => {
  test('a successful login redirects home', async ({ page }) => {
    await page.route(`${API}/auth/login`, (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ session: 'irrelevant-now' }) });
    });

    await page.goto('/login');
    await page.fill('#username', 'someone');
    await page.fill('#password', 'correct-password');
    await page.click('button[type="submit"]');

    await page.waitForURL((url) => url.pathname === '/');
  });

  test('sends email vs username depending on the input', async ({ page }) => {
    let sentBody = null;
    await page.route(`${API}/auth/login`, (route) => {
      sentBody = route.request().postDataJSON();
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) });
    });

    await page.goto('/login');
    await page.fill('#username', 'someone@example.com');
    await page.fill('#password', 'hunter2');
    await page.click('button[type="submit"]');

    await page.waitForURL((url) => url.pathname === '/');
    expect(sentBody).toEqual({ password: 'hunter2', email: 'someone@example.com' });
  });

  test('a failed login shows the error and stays on the page', async ({ page }) => {
    await page.route(`${API}/auth/login`, (route) => {
      route.fulfill({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'Invalid username or password' }) });
    });

    let alertMessage = null;
    page.on('dialog', async (d) => {
      alertMessage = d.message();
      await d.dismiss();
    });

    await page.goto('/login');
    await page.fill('#username', 'someone');
    await page.fill('#password', 'wrong-password');
    await page.click('button[type="submit"]');

    await expect.poll(() => alertMessage).toBe('Invalid username or password');
    expect(new URL(page.url()).pathname).toBe('/login');
  });

});

test.describe('logout', () => {
  test('a successful logout redirects home', async ({ page }) => {
    await page.route(`${API}/auth/logout`, (route) => {
      expect(route.request().method()).toBe('POST');
      route.fulfill({ status: 204 });
    });

    await page.goto('/');
    await page.evaluate(() => logout());

    await page.waitForURL((url) => url.pathname === '/');
  });

  test('a failed logout does not navigate away', async ({ page }) => {
    await page.route(`${API}/auth/logout`, (route) => {
      route.fulfill({ status: 500, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'boom' }) });
    });

    await page.goto('/login');
    await page.evaluate(() => logout());
    await page.waitForTimeout(300);

    expect(new URL(page.url()).pathname).toBe('/login');
  });
});
