import { test, expect, signIn, error } from './helpers.js';

const LINKS = [
  { platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true },
  { platform: 'twitch', platform_username: 'streamer99', verified: false, login_enabled: false },
];

const loginPage = (page) => page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));

test.describe('account page', () => {
  test('FS-01 the account page shows the profile and a row for each platform from the API', async ({ page }) => {
    await signIn(page, { account: { username: 'testuser', password_auth: true }, myLinks: LINKS });
    await page.goto('/account');

    await expect(page.locator('#account-username')).toHaveText('testuser');
    await expect(page.locator('#link-discord-title')).toHaveText('someone#1234');
    await expect(page.locator('#link-discord-action')).toHaveText('Unlink');
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
    await expect(page.locator('#link-twitch-status')).toHaveText('Unverified');
    await expect(page.locator('#link-twitch-login-enabled')).toBeDisabled();
    await expect(page.locator('#link-microsoft-title')).toHaveText('Microsoft');
    await expect(page.locator('#link-microsoft-action')).toHaveText('Link');
    await expect(page.locator('#link-steam-title')).toHaveText('Steam');
    await expect(page.locator('#link-steam-action')).toHaveText('Link');
  });

  test('FS-02 unlinking asks first, then sends a DELETE and shows the row unlinked', async ({ page }) => {
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

  test('FS-03 a refused unlink shows the API message and leaves the row linked', async ({ page }) => {
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

  test('FS-04 toggling a platform login off and on sends a PATCH each time and the box follows', async ({ page }) => {
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

  test('FS-05 a refused platform login toggle puts the box back and shows the API message', async ({ page }) => {
    await signIn(page, {
      myLinks: LINKS,
      failures: { 'PATCH /users/me/link/discord': { status: 409, detail: 'Keep one way to sign in', times: 1 } },
    });
    await page.goto('/account');
    await page.locator('label[for="link-discord-login-enabled"]').click();
    await expect(error(page)).toHaveText('Keep one way to sign in');
    await expect(page.locator('#link-discord-login-enabled')).toBeChecked();
  });

  test('FS-06 the password login toggle shows the loaded setting and sends a PATCH each time', async ({ page }) => {
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

  test('FS-07 a keyboard toggle while the first one is in flight is undone and says it was not sent', async ({ page }) => {
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
    await expect(toggle).not.toBeChecked();
    await expect(error(page)).toHaveText('The change made while saving was not sent. Make it again.');
    expect(await writes()).toEqual([{ method: 'PATCH', path: '/users/me/settings', body: { password_auth: false } }]);
  });

  test('FS-08 a refused password login toggle puts the box back and shows the API message', async ({ page }) => {
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

test.describe('admin access', () => {
  test('FS-10 a signed-out visitor is sent to the login page', async ({ page }) => {
    await loginPage(page);
    await page.goto('/admin/users');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('FS-12 going back after the session ends does not show the page again', async ({ page }) => {
    await signIn(page);
    await loginPage(page);
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-roles li').first()).toBeVisible();
    await page.goto('/admin');
    await expect(page.locator('#admin-users-link')).toBeVisible();
    await page.context().clearCookies();
    await page.goBack();
    await expect(page).toHaveURL(/\/login$/);
  });

  test('FS-13 a change made after the session ends sends the visitor to the login page', async ({ page }) => {
    await signIn(page);
    await loginPage(page);
    await page.goto('/admin/permissions');
    await expect(page.locator('#admin-permissions li').first()).toBeVisible();
    await page.context().clearCookies();
    page.once('dialog', (dialog) => dialog.accept());
    await page.locator('#admin-permissions li').first().getByRole('button', { name: 'Delete' }).click();
    await expect(page).toHaveURL(/\/login$/);
  });

  test('FS-14 the bee page has no review link without the bee admin permission', async ({ page }) => {
    await signIn(page, { me: ['users.admin', 'roles.admin'] });
    const answered = page.waitForResponse('**/project/bee-name-generator/admin-link');
    await page.goto('/project/bee-name-generator');
    await answered;
    await expect(page.locator('[hx-get="/project/bee-name-generator/admin-link"]')).toHaveCount(0);
    await expect(page.locator('#bee-admin-link')).toHaveCount(0);
  });

  test('FS-15 the bee page shows a review link with the bee admin permission and it opens the review page', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto('/project/bee-name-generator');
    await page.locator('#bee-admin-link').click();
    await expect(page).toHaveURL(/\/project\/bee-name-generator\/admin$/);
    await expect(page.locator('#bee-suggestions li')).toHaveCount(3);
  });
});

test.describe('bee review', () => {
  const rows = (page) => page.locator('#bee-suggestions li');
  const review = '/project/bee-name-generator/admin';

  test('FS-20 the review page lists the pending suggestions', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto(review);
    await expect(rows(page)).toHaveText([/buzz/, /honey/, /wax/]);
  });

  test('FS-21 accepting sends a PUT for the exact name, removes the row and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto(review);
    await page.getByRole('button', { name: 'Accept honey' }).click();
    await expect(rows(page)).toHaveText([/buzz/, /wax/]);
    expect(await writes()).toEqual([{ method: 'PUT', path: '/bee-name-generator/suggestion/honey', body: null }]);
    await expect(page.locator('#bee-suggestions-title')).toBeFocused();
  });

  test('FS-22 rejecting sends a DELETE for a name that needs escaping and shows the empty list', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['beenamegenerator.admin'], suggestions: ['royal jelly/queen?'] });
    await page.goto(review);
    await page.getByRole('button', { name: 'Reject royal jelly/queen?' }).click();
    await expect(page.locator('#bee-suggestions-empty')).toHaveText('No pending suggestions');
    expect(await writes()).toEqual([{ method: 'DELETE', path: '/bee-name-generator/suggestion/royal%20jelly%2Fqueen%3F', body: null }]);
  });

  test('FS-23 a refused review shows the API message and keeps the row, and the next try works', async ({ page }) => {
    await signIn(page, {
      me: ['beenamegenerator.admin'],
      failures: { 'PUT /bee-name-generator/suggestion/buzz': { status: 500, detail: 'Failed to accept', times: 1 } },
    });
    await page.goto(review);
    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(error(page)).toHaveText('Failed to accept');
    await expect(rows(page)).toHaveCount(3);

    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(rows(page)).toHaveCount(2);
    await expect(error(page)).toHaveText('');
  });
});

test.describe('bee page', () => {
  const NAME_API = '**/api/v1/bee-name-generator/name';
  const SUGGEST_API = '**/api/v1/bee-name-generator/suggestion/**';
  const cors = { 'access-control-allow-origin': '*' };
  const nameStatus = (page) => page.locator('#get-bee-status');
  const suggestStatus = (page) => page.locator('#suggest-bee-status');
  const submit = (page, text) => page.locator('#suggest-bee-name').fill(text).then(() => page.locator('#suggest-bee-button').click());

  /** The browser sends no cookie and the stub sends no CORS headers, so the suggestion call is forwarded with the session. */
  const forwardSuggestions = (page, session) =>
    page.route(SUGGEST_API, async (route) => {
      const response = await route.fetch({ headers: { cookie: `session=${session}` } });
      await route.fulfill({ response, headers: { ...response.headers(), ...cors } });
    });

  test('FS-24 generating disables the button while the request is held, shows the returned name and enables the button again', async ({ page }) => {
    await signIn(page);
    let release;
    const held = new Promise((resolve) => (release = resolve));
    await page.route(NAME_API, async (route) => {
      await held;
      await route.fulfill({ status: 200, contentType: 'application/json', headers: cors, body: JSON.stringify({ name: 'Queen Buzzabelle' }) });
    });
    await page.goto('/project/bee-name-generator');
    await expect(page.locator('#get-bee-button')).toBeEnabled();

    await page.locator('#get-bee-button').click();
    await expect(page.locator('#get-bee-button')).toBeDisabled();
    await expect(page.locator('#get-bee-name')).toHaveText('\u2014');

    release();
    await expect(page.locator('#get-bee-name')).toHaveText('Queen Buzzabelle');
    await expect(page.locator('#get-bee-button')).toBeEnabled();
    await expect(nameStatus(page)).toBeHidden();
  });

  test('FS-25 a failing answer shows the retry message, keeps the name and enables the button again', async ({ page }) => {
    await signIn(page);
    const answers = [{ status: 200, body: { name: 'Queen Buzzabelle' } }, { status: 500, body: { detail: 'down' } }];
    await page.route(NAME_API, (route) => {
      const { status, body } = answers.shift();
      return route.fulfill({ status, contentType: 'application/json', headers: cors, body: JSON.stringify(body) });
    });
    await page.goto('/project/bee-name-generator');
    await page.locator('#get-bee-button').click();
    await expect(page.locator('#get-bee-name')).toHaveText('Queen Buzzabelle');

    await page.locator('#get-bee-button').click();
    await expect(nameStatus(page)).toHaveText("Couldn't get a name, please try again.");
    await expect(page.locator('#get-bee-name')).toHaveText('Queen Buzzabelle');
    await expect(page.locator('#get-bee-button')).toBeEnabled();
  });

  test('FS-26 a name that needs escaping is sent as a POST to the escaped path, thanks the visitor and clears the input', async ({ page }) => {
    const { session, writes } = await signIn(page, { me: ['beenamegenerator.admin'], suggestions: [] });
    await forwardSuggestions(page, session);
    await page.goto('/project/bee-name-generator');

    await submit(page, 'royal jelly/queen?');
    await expect(suggestStatus(page)).toHaveText('Thanks for the suggestion!');
    await expect(page.locator('#suggest-bee-name')).toHaveValue('');
    await expect(page.locator('#suggest-bee-button')).toBeEnabled();
    expect(await writes()).toEqual([{ method: 'POST', path: '/bee-name-generator/suggestion/royal%20jelly%2Fqueen%3F', body: null }]);
  });

  test('FS-27 a refused suggestion shows the retry message and keeps the typed name', async ({ page }) => {
    const { session } = await signIn(page, {
      me: ['beenamegenerator.admin'],
      failures: { 'POST /bee-name-generator/suggestion/wax': { status: 500, detail: 'Failed to save' } },
    });
    await forwardSuggestions(page, session);
    await page.goto('/project/bee-name-generator');

    await submit(page, 'wax');
    await expect(suggestStatus(page)).toHaveText("Couldn't submit that, please try again.");
    await expect(page.locator('#suggest-bee-name')).toHaveValue('wax');
    await expect(page.locator('#suggest-bee-button')).toBeEnabled();
  });

  test('FS-27 an unreachable API shows the retry message and keeps the typed name', async ({ page }) => {
    await signIn(page);
    await page.route(SUGGEST_API, (route) => route.abort('connectionrefused'));
    await page.goto('/project/bee-name-generator');

    await submit(page, 'wax');
    await expect(suggestStatus(page)).toHaveText("Couldn't submit that, please try again.");
    await expect(page.locator('#suggest-bee-name')).toHaveValue('wax');
    await expect(page.locator('#suggest-bee-button')).toBeEnabled();
  });

  for (const [label, text] of [['an empty name', ''], ['a whitespace name', '   '], ['a single dot', '.'], ['a double dot', '..']]) {
    test(`FS-28 ${label} shows the invalid message and sends no request`, async ({ page }) => {
      const { writes } = await signIn(page, { me: ['beenamegenerator.admin'] });
      const sent = [];
      await page.route(SUGGEST_API, (route) => {
        sent.push(route.request().url());
        return route.abort();
      });
      await page.goto('/project/bee-name-generator');
      // The input is required, so an empty value is stopped by the browser before the script runs.
      await page.locator('form[onsubmit]').evaluate((form) => (form.noValidate = true));

      await submit(page, text);
      await expect(suggestStatus(page)).toHaveText('Please enter a valid name.');
      await expect(page.locator('#suggest-bee-button')).toBeEnabled();
      expect(sent).toEqual([]);
      expect(await writes()).toEqual([]);
    });
  }
});
