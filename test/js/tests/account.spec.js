import { test, expect, signIn, HOSTILE, error, expectNoInjection } from './helpers.js';

const LINKS = [
  { platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true },
  { platform: 'twitch', platform_username: 'streamer99', verified: false, login_enabled: false },
];

test.describe('account page - loading', () => {
  test('renders profile and linked-account rows', async ({ page }) => {
    await signIn(page, { account: { username: 'testuser', password_auth: true }, myLinks: LINKS });
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
    await signIn(page);
    // Intercept rather than let the browser actually reach Steam - only the
    // request URL buildSteamOpenIDURL() produced matters here.
    const steamUrl = process.env.STEAM_OPENID_LOGIN_URL;
    await page.route(`${steamUrl}*`, (route) => {
      route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' });
    });

    await page.goto('/account');
    await page.locator('#link-steam-action').click();
    await page.waitForURL((url) => url.href.startsWith(steamUrl));

    const url = new URL(page.url());
    expect(url.searchParams.get('openid.mode')).toBe('checkid_setup');
    const returnTo = new URL(url.searchParams.get('openid.return_to'));
    const state = JSON.parse(
      Buffer.from(returnTo.searchParams.get('state').replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8')
    );
    expect(state.platform).toBe('steam');
    expect(state.mode).toBe('link');
  });

  test('clicking Link on Discord navigates to the OAuth URL with the link state', async ({ page }) => {
    await signIn(page);
    await page.route('https://discord.com/**', (route) => route.fulfill({ status: 200, contentType: 'text/plain', body: 'stub' }));
    await page.goto('/account');
    await page.locator('#link-discord-action').click();
    await page.waitForURL((url) => url.hostname === 'discord.com');
    const state = JSON.parse(
      Buffer.from(new URL(page.url()).searchParams.get('state').replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8')
    );
    expect(state.platform).toBe('discord');
    expect(state.mode).toBe('link');
  });

  test('account text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, {
      account: { username: HOSTILE, password_auth: true },
      myLinks: [{ platform: 'discord', platform_username: HOSTILE, verified: true, login_enabled: true }],
    });
    await page.goto('/account');
    await expect(page.locator('#account-username')).toHaveText(HOSTILE);
    await expect(page.locator('#link-discord-title')).toHaveText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('account page - unlink', () => {
  test('unlinking asks first, then round-trips and refreshes the row', async ({ page }) => {
    const { writes } = await signIn(page, { myLinks: LINKS });
    const prompts = [];
    page.on('dialog', (dialog) => {
      prompts.push(dialog.message());
      return prompts.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto('/account');

    await page.locator('#link-discord-action').click();
    await expect.poll(() => prompts).toEqual(['Unlink Discord from your account?']);
    expect(await writes()).toEqual([]);

    await page.locator('#link-discord-action').click();
    await expect(page.locator('#link-discord-action')).toHaveText('Link');
    await expect(page.locator('#link-discord-title')).toHaveText('Discord');
    await expect(page.locator('#link-discord-login-enabled')).toBeDisabled();
    expect(await writes()).toEqual([{ method: 'DELETE', path: '/users/me/link/discord', body: null }]);
  });

  test('a failed unlink shows the error and leaves the row linked', async ({ page }) => {
    await signIn(page, {
      myLinks: LINKS,
      failures: { 'DELETE /users/me/link/discord': { status: 409, detail: 'Keep one way to sign in', times: 1 } },
    });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/account');
    await page.locator('#link-discord-action').click();
    await expect(error(page)).toHaveText('Keep one way to sign in');
    await expect(page.locator('#link-discord-action')).toHaveText('Unlink');
    await expect(page.locator('#link-discord-title')).toHaveText('someone#1234');
  });
});

test.describe('account page - login-enabled toggle', () => {
  test('toggling off then on round-trips and refreshes the row', async ({ page }) => {
    const { writes } = await signIn(page, { myLinks: LINKS });
    await page.goto('/account');
    await page.locator('label[for="link-discord-login-enabled"]').click();
    await expect.poll(writes).toEqual([{ method: 'PATCH', path: '/users/me/link/discord', body: { login_enabled: false } }]);
    await expect(page.locator('#link-discord-login-enabled')).not.toBeChecked();
    await expect(page.locator('#link-discord[aria-busy="true"]')).toHaveCount(0);

    await page.locator('label[for="link-discord-login-enabled"]').click();
    await expect.poll(async () => (await writes()).length).toBe(2);
    expect((await writes())[1]).toEqual({ method: 'PATCH', path: '/users/me/link/discord', body: { login_enabled: true } });
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
  });

  test('a failed toggle puts the checkbox back and shows the API message', async ({ page }) => {
    await signIn(page, {
      myLinks: LINKS,
      failures: { 'PATCH /users/me/link/discord': { status: 409, detail: 'Keep one way to sign in', times: 1 } },
    });
    await page.goto('/account');
    await page.locator('label[for="link-discord-login-enabled"]').click();
    await expect(error(page)).toHaveText('Keep one way to sign in');
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
  });

  test('a second change to a row while one is in flight is dropped', async ({ page }) => {
    const { writes, gate } = await signIn(page, { myLinks: LINKS });
    await page.goto('/account');
    const patch = await gate('PATCH /users/me/link/discord');
    await page.locator('label[for="link-discord-login-enabled"]').click();
    await patch.arrived();
    await expect(page.locator('#link-discord[aria-busy="true"]')).toHaveCount(1);
    await page.locator('#link-discord-login-enabled').dispatchEvent('click');
    await patch.release();
    await expect(page.locator('#link-discord[aria-busy="true"]')).toHaveCount(0);
    expect(await writes()).toEqual([{ method: 'PATCH', path: '/users/me/link/discord', body: { login_enabled: false } }]);
    await expect(page.locator('#link-discord-login-enabled')).not.toBeChecked();
  });
});

test.describe('account page - password login toggle', () => {
  test('reflects the loaded setting and round-trips a toggle', async ({ page }) => {
    const { writes } = await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    await expect(page.locator('#password-auth-enabled')).toBeChecked();

    await page.locator('label[for="password-auth-enabled"]').click();
    await expect.poll(writes).toEqual([{ method: 'PATCH', path: '/users/me/settings', body: { password_auth: false } }]);
    await expect(page.locator('#password-auth-enabled')).not.toBeChecked();
    await expect(page.locator('#account-password[aria-busy="true"]')).toHaveCount(0);

    await page.locator('label[for="password-auth-enabled"]').click();
    await expect.poll(async () => (await writes()).length).toBe(2);
    expect((await writes())[1].body).toEqual({ password_auth: true });
    await expect(page.locator('#password-auth-enabled')).toBeChecked();
  });

  test('a keyboard toggle while the first one is in flight is undone and says it was not sent', async ({ page }) => {
    const { writes, gate } = await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    const patch = await gate('PATCH /users/me/settings');
    const toggle = page.locator('#password-auth-enabled');
    await toggle.focus();
    await page.keyboard.press('Space');
    await patch.arrived();
    await expect(page.locator('#account-password')).toHaveAttribute('aria-busy', 'true');
    await expect(toggle).not.toBeChecked();
    await page.keyboard.press('Space');
    await expect(error(page)).toHaveText('The last change is still being saved. Try again in a moment.');
    await expect(toggle).not.toBeChecked();
    await patch.release();
    await expect(page.locator('#account-password')).not.toHaveAttribute('aria-busy', 'true');
    await expect(page.locator('#password-auth-enabled')).not.toBeChecked();
    await expect(error(page)).toHaveText('The change made while saving was not sent. Make it again.');
    expect(await writes()).toEqual([{ method: 'PATCH', path: '/users/me/settings', body: { password_auth: false } }]);
  });

  test('a rejected toggle puts the checkbox back and shows the API detail', async ({ page }) => {
    await signIn(page, {
      account: { username: 'testuser', password_auth: true },
      failures: { 'PATCH /users/me/settings': { status: 409, detail: 'Link another sign-in method first', times: 1 } },
    });
    await page.goto('/account');
    await page.locator('label[for="password-auth-enabled"]').click();
    await expect(error(page)).toHaveText('Link another sign-in method first');
    await expect(page.locator('#password-auth-enabled')).toBeChecked();
  });
});
