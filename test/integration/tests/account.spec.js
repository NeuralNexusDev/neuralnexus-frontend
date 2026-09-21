import { test, expect } from '@playwright/test';

const API = `${process.env.API_BASE_URL}/api/v1`;

async function mockMe(page, { username = 'testuser', status = 200 } = {}) {
  await page.route(`${API}/users/me`, (route) => {
    route.fulfill({ status, contentType: 'application/json', body: JSON.stringify({ username }) });
  });
}

async function mockLinks(page, links) {
  await page.route(`${API}/users/me/links`, (route) => {
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(links) });
  });
}

test.describe('account page - loading', () => {
  test('renders profile and linked-account rows', async ({ page }) => {
    await mockMe(page, { username: 'testuser' });
    await mockLinks(page, [
      { platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true },
      { platform: 'twitch', platform_username: 'streamer99', verified: false, login_enabled: false },
    ]);

    await page.goto('/account');

    await expect(page.locator('#account-username')).toHaveText('testuser');
    await expect(page.locator('#link-discord-title')).toHaveText('someone#1234');
    await expect(page.locator('#link-discord-action')).toHaveText('Unlink');
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();

    // Unverified: checkbox is disabled and shows the "Unverified" status.
    await expect(page.locator('#link-twitch-status')).toHaveText('Unverified');
    await expect(page.locator('#link-twitch-login-enabled')).toBeDisabled();

    // Never linked: falls back to the platform's display name as the title.
    await expect(page.locator('#link-microsoft-title')).toHaveText('Microsoft');
    await expect(page.locator('#link-microsoft-action')).toHaveText('Link');

    await expect(page.locator('#link-steam-title')).toHaveText('Steam');
    await expect(page.locator('#link-steam-action')).toHaveText('Link');
  });

  test('clicking Link on Steam navigates to a Steam OpenID URL, not a pre-rendered base href', async ({ page }) => {
    await mockMe(page, { username: 'testuser' });
    await mockLinks(page, []);
    // Intercept rather than let the browser actually reach Steam - only the
    // request URL buildSteamOpenIDURL() produced matters here.
    await page.route('https://steamcommunity.com/openid/login*', (route) => {
      route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' });
    });

    await page.goto('/account');
    await page.locator('#link-steam-action').click();
    await page.waitForURL((url) => url.hostname === 'steamcommunity.com');

    const url = new URL(page.url());
    expect(url.searchParams.get('openid.mode')).toBe('checkid_setup');
    const returnTo = new URL(url.searchParams.get('openid.return_to'));
    const state = JSON.parse(
      Buffer.from(returnTo.searchParams.get('state').replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8')
    );
    expect(state.platform).toBe('steam');
    expect(state.mode).toBe('link');
  });

  test.describe('401 handling', () => {
    test('redirects to /login when /me is unauthorized', async ({ page }) => {
      await mockMe(page, { status: 401 });
      await mockLinks(page, []);
      await page.goto('/account');
      await page.waitForURL('**/login');
    });

    test('redirects to /login when /me/links is unauthorized', async ({ page }) => {
      await mockMe(page, { username: 'testuser' });
      await page.route(`${API}/users/me/links`, (route) => {
        route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'Unauthorized' }) });
      });
      await page.goto('/account');
      await page.waitForURL('**/login');
    });
  });
});

test.describe('account page - unlink', () => {
  test('unlinking a platform round-trips and refreshes the row', async ({ page }) => {
    await mockMe(page);
    let linked = true;
    await page.route(`${API}/users/me/links`, (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(linked ? [{ platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true }] : []),
      });
    });
    await page.route(`${API}/users/me/link/discord`, (route) => {
      expect(route.request().method()).toBe('DELETE');
      linked = false;
      route.fulfill({ status: 204 });
    });

    await page.goto('/account');
    await expect(page.locator('#link-discord-action')).toHaveText('Unlink');

    page.once('dialog', (d) => d.accept());
    await page.locator('#link-discord-action').click();

    await expect(page.locator('#link-discord-action')).toHaveText('Link');
  });

  test('a failed unlink shows the error and leaves the row linked', async ({ page }) => {
    await mockMe(page);
    await mockLinks(page, [{ platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true }]);
    await page.route(`${API}/users/me/link/discord`, (route) => {
      route.fulfill({ status: 500, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'something broke' }) });
    });

    await page.goto('/account');

    let alertMessage = null;
    page.on('dialog', async (d) => {
      if (d.type() === 'confirm') {
        await d.accept();
      } else {
        alertMessage = d.message();
        await d.dismiss();
      }
    });
    await page.locator('#link-discord-action').click();

    await expect.poll(() => alertMessage).toBe('something broke');
    await expect(page.locator('#link-discord-action')).toHaveText('Unlink');
  });
});

test.describe('account page - login-enabled toggle', () => {
  test('toggling on round-trips and refreshes the row', async ({ page }) => {
    await mockMe(page);
    let enabled = false;
    await page.route(`${API}/users/me/links`, (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([{ platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: enabled }]),
      });
    });
    await page.route(`${API}/users/me/link/discord`, (route) => {
      expect(route.request().method()).toBe('PATCH');
      expect(route.request().postDataJSON()).toEqual({ login_enabled: true });
      enabled = true;
      route.fulfill({ status: 204 });
    });

    await page.goto('/account');
    await expect(page.locator('#link-discord-login-enabled')).not.toBeChecked();

    // The checkbox is visually-hidden (sr-only); its wrapping <label> is
    // what actually receives the click, same as a real user interaction.
    await page.locator('#link-discord-login-enabled').locator('..').click();

    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
  });

  test('a failed toggle reverts the checkbox and alerts', async ({ page }) => {
    await mockMe(page);
    await mockLinks(page, [{ platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: false }]);
    await page.route(`${API}/users/me/link/discord`, (route) => {
      route.fulfill({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'nope' }) });
    });

    await page.goto('/account');

    let alertMessage = null;
    page.on('dialog', async (d) => {
      alertMessage = d.message();
      await d.dismiss();
    });

    await page.evaluate(() => setPlatformLoginEnabled('discord', true));

    await expect.poll(() => alertMessage).toBe('nope');
    await expect(page.locator('#link-discord-login-enabled')).not.toBeChecked();
  });

  test('401 on toggle redirects to /login instead of alerting', async ({ page }) => {
    await mockMe(page);
    await mockLinks(page, [{ platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true }]);
    await page.route(`${API}/users/me/link/discord`, (route) => {
      route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'Unauthorized' }) });
    });

    await page.goto('/account');
    await page.evaluate(() => setPlatformLoginEnabled('discord', false));
    await page.waitForURL('**/login');
  });
});

test.describe('account page - request-sequencing regressions', () => {
  // These formalize the races found and fixed during review: a slow,
  // out-of-order response from a superseded action must never win over a
  // newer one, whether that's two toggles on different platforms, two
  // toggles on the same platform, or a toggle followed by an unlink.

  test('a stale cross-platform refresh does not clobber a newer one', async ({ page }) => {
    await mockMe(page);
    let linksCallCount = 0;
    await page.route(`${API}/users/me/links`, async (route) => {
      linksCallCount++;
      if (linksCallCount === 1) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            { platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: false },
            { platform: 'twitch', platform_username: 't1', verified: true, login_enabled: false },
          ]),
        });
      } else if (linksCallCount === 2) {
        // Discord's refresh: slow and stale (only discord flipped).
        await new Promise((r) => setTimeout(r, 400));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            { platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: true },
            { platform: 'twitch', platform_username: 't1', verified: true, login_enabled: false },
          ]),
        });
      } else {
        // Twitch's refresh: fast and fresh (both flipped).
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            { platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: true },
            { platform: 'twitch', platform_username: 't1', verified: true, login_enabled: true },
          ]),
        });
      }
    });
    for (const platform of ['discord', 'twitch']) {
      await page.route(`${API}/users/me/link/${platform}`, (route) => route.fulfill({ status: 204 }));
    }

    await page.goto('/account');
    await page.evaluate(() => setPlatformLoginEnabled('discord', true));
    await page.waitForTimeout(50);
    await page.evaluate(() => setPlatformLoginEnabled('twitch', true));

    // Wait past discord's slow, stale refresh before asserting final state -
    // toBeChecked() only waits for "becomes true", it won't catch a later
    // response reverting it back after this point.
    await page.waitForTimeout(600);
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
    await expect(page.locator('#link-twitch-login-enabled')).toBeChecked();
  });

  test('a stale same-platform double-toggle does not clobber the real result', async ({ page }) => {
    await mockMe(page);
    await mockLinks(page, [{ platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: true }]);

    let alerted = null;
    page.on('dialog', async (d) => { alerted = d.message(); await d.dismiss(); });

    let patchCount = 0;
    await page.route(`${API}/users/me/link/discord`, async (route) => {
      patchCount++;
      if (patchCount === 1) {
        // First click (turn off): slow, and the server rejects it.
        await new Promise((r) => setTimeout(r, 300));
        await route.fulfill({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'stale click rejected' }) });
      } else {
        // Second click (turn back on): fast, succeeds.
        await route.fulfill({ status: 204 });
      }
    });

    await page.goto('/account');
    await page.evaluate(() => setPlatformLoginEnabled('discord', false));
    await page.waitForTimeout(20);
    await page.evaluate(() => setPlatformLoginEnabled('discord', true));

    await expect(page.locator('#link-discord-login-enabled')).toBeChecked({ timeout: 2000 });
    expect(alerted).toBeNull();
  });

  test('a toggle superseded by an unlink does not pop a stale-failure alert', async ({ page }) => {
    await mockMe(page);
    let linksCallCount = 0;
    await page.route(`${API}/users/me/links`, async (route) => {
      linksCallCount++;
      if (linksCallCount === 1) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([{ platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: false }]),
        });
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
      }
    });
    await page.route(`${API}/users/me/link/discord`, async (route) => {
      if (route.request().method() === 'PATCH') {
        // Orphaned by the unlink below; fails slowly, after the unlink succeeds.
        await new Promise((r) => setTimeout(r, 300));
        await route.fulfill({ status: 404, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'link not found' }) });
      } else {
        await route.fulfill({ status: 204 });
      }
    });

    let alerted = null;
    page.on('dialog', async (d) => {
      if (d.type() === 'confirm') {
        await d.accept();
      } else {
        alerted = d.message();
        await d.dismiss();
      }
    });

    await page.goto('/account');
    await page.evaluate(() => setPlatformLoginEnabled('discord', true));
    await page.waitForTimeout(50);
    await page.evaluate(() => unlinkPlatform('discord'));

    await expect(page.locator('#link-discord-action')).toHaveText('Link', { timeout: 2000 });
    await page.waitForTimeout(400); // let the orphaned PATCH's late failure resolve
    expect(alerted).toBeNull();
  });

  test('a stale toggle success never re-fetches once superseded by an unlink', async ({ page }) => {
    // Guards the fix's own safety margin: even though a fresh
    // loadLinkedAccounts() call always wins by recency, a stale toggle's
    // now-pointless success is gated so it never triggers a redundant
    // (and, if the backend were ever inconsistent, risky) extra refetch.
    await mockMe(page);
    let linksCallCount = 0;
    await page.route(`${API}/users/me/links`, async (route) => {
      linksCallCount++;
      if (linksCallCount === 1) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([{ platform: 'discord', platform_username: 'd#1', verified: true, login_enabled: false }]),
        });
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
      }
    });
    await page.route(`${API}/users/me/link/discord`, async (route) => {
      if (route.request().method() === 'DELETE') {
        await route.fulfill({ status: 204 });
      } else {
        // Orphaned toggle: succeeds, but late (after the unlink).
        await new Promise((r) => setTimeout(r, 300));
        await route.fulfill({ status: 204 });
      }
    });

    page.on('dialog', async (d) => {
      if (d.type() === 'confirm') await d.accept();
      else await d.dismiss();
    });

    await page.goto('/account');
    await page.evaluate(() => setPlatformLoginEnabled('discord', true));
    await page.waitForTimeout(50);
    await page.evaluate(() => unlinkPlatform('discord'));

    await expect(page.locator('#link-discord-action')).toHaveText('Link', { timeout: 2000 });
    await page.waitForTimeout(400); // let the orphaned toggle's late success resolve
    expect(linksCallCount).toBe(2); // initial load + the unlink's own refresh only
  });
});
