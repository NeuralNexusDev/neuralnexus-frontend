import { randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

const STUB = process.env.NN_API_URL;
const APP = process.env.BASE_URL || 'http://localhost:8099';
const HTMX_CDN = 'https://cdn.neuralnexus.dev/htmx/htmx.v1.9.5.min.js';
const HTMX_FILE = fileURLToPath(new URL('../node_modules/htmx.org/dist/htmx.min.js', import.meta.url));

// Snowflake-sized IDs, above 2^53, matching the ones stub-admin.mjs seeds.
const ID = {
  system: '3541025163146757610',
  owner: '3541025163146757620',
  bee: '3541025163146757630',
  pBee: '3541025163146757710',
  pRate: '3541025163146757730',
  pPets: '3541025163146757780',
  pMotd: '3541025163146757790',
  pStore: '3541025163146757800',
  alice: '3541025163146759010',
  bob: '3541025163146759020',
  anon: '3541025163146759030',
};

const HOSTILE = '"><img src=x onerror="window.__xss=1">';

const test = base.extend({
  page: async ({ page }, use) => {
    await page.route(HTMX_CDN, (route) => route.fulfill({ contentType: 'application/javascript', path: HTMX_FILE }));
    await use(page);
  },
});

/** Seeds a session on the stub API and signs the page in with it, since the app forwards that cookie to the API. */
async function signIn(page, state = {}) {
  const session = randomUUID();
  const seeded = await page.request.post(`${STUB}/__admin/state`, { data: { session, state } });
  expect(seeded.ok()).toBe(true);
  await page.context().addCookies([{ name: 'session', value: session, url: APP }]);
  const calls = async () => (await page.request.get(`${STUB}/__admin/calls?session=${session}`)).json();
  const writes = async () =>
    (await calls())
      .filter((call) => call.method !== 'GET')
      .map(({ method, path, body }) => ({ method, path, body: body ?? null }));
  const api = (method, path, data) =>
    page.request.fetch(`${STUB}/api/v1${path}`, { method, data, headers: { Cookie: `session=${session}` } });
  return { session, calls, writes, api };
}

async function expectNoInjection(page) {
  await expect(page.locator('img[src="x"]')).toHaveCount(0);
  expect(await page.evaluate(() => window.__xss)).toBeUndefined();
}

const error = (page) => page.locator('#admin-error');

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

  test('admin pages are served with no-store', async ({ page }) => {
    await signIn(page);
    for (const path of ['/admin', '/admin/users', '/admin/roles', '/admin/permissions']) {
      const res = await page.request.get(`${APP}${path}`);
      expect(res.status()).toBe(200);
      expect(res.headers()['cache-control']).toBe('no-store');
    }
  });

  test('a server that cannot be reached shows a message in the banner', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await page.route('**/admin/permissions/*', (route) => route.abort('failed'));
    page.once('dialog', (dialog) => dialog.accept());
    await page.locator('#admin-permissions li').first().getByRole('button', { name: 'Delete' }).click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
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

test.describe('admin - user list', () => {
  const rows = (page) => page.locator('#admin-users > li:has(> a)');
  const search = (page) => page.locator('#admin-users-search');

  test('lists every user with a link to its editor and role names', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(3);
    await expect(rows(page).nth(0)).toContainText('alice');
    await expect(rows(page).nth(0)).toContainText(ID.alice);
    await expect(rows(page).nth(0).locator('a')).toHaveAttribute('href', `/admin/users/${ID.alice}`);
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText(['bee_admin']);
    await expect(rows(page).nth(2)).toContainText('No username');
  });

  test('the search box asks the server, matching username or user ID, ignoring case and surrounding spaces', async ({ page }) => {
    const { calls } = await signIn(page);
    await page.goto('/admin/users');
    await search(page).fill('  BO ');
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('bob');

    await search(page).fill('675903');
    await expect(rows(page)).toContainText('No username');

    await search(page).fill('nobody');
    await expect(rows(page)).toHaveCount(0);
    await expect(page.locator('#admin-users-empty')).toHaveText('No users found');

    await search(page).fill('');
    await expect(rows(page)).toHaveCount(3);
    await expect(page.locator('#admin-users-empty')).toHaveCount(0);
    await expect(search(page)).toHaveValue('');
    expect((await calls()).filter((call) => call.path === '/users').length).toBeGreaterThan(1);
  });

  test('a search covers users beyond the first page, and stops asking once the list ends', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await search(page).fill('user247');
    await expect(rows(page)).toHaveCount(1);
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
  });

  test('a search that reaches its limit offers to keep looking, and keeps the search text', async ({ page }) => {
    await signIn(page, { generateUsers: 1100 });
    await page.goto('/admin/users');
    await search(page).fill('user1099');
    await expect(page.locator('#admin-users-empty')).toHaveText('No matches in the first 1000 users');
    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('user1099');
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
    await expect(search(page)).toHaveValue('user1099');
  });

  test('Load more adds the next page and the button goes when the list ends', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(250);
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
  });

  test('a failed Load more shows the API message, keeps the list, and a retry clears the message', async ({ page }) => {
    await signIn(page, { generateUsers: 247, failures: { 'GET /users': { status: 500, detail: 'Failed to list users', skip: 1, times: 1 } } });
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(error(page)).toHaveText('Failed to list users');
    await expect(rows(page)).toHaveCount(200);

    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(250);
    await expect(error(page)).toHaveText('');
  });

  test('role IDs are shown when the caller cannot read roles', async ({ page }) => {
    await signIn(page, { me: ['users.admin'] });
    await page.goto('/admin/users');
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText([ID.bee]);
  });

  test('a refused list shows the API message and no search box', async ({ page }) => {
    await signIn(page, { me: ['roles.admin'] });
    await page.goto('/admin/users');
    await expect(error(page)).toHaveText('You do not have permission to manage users');
    await expect(page.locator('#admin-users-search')).toHaveCount(0);
  });

  test('usernames are rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, { users: [{ user_id: ID.bob, username: HOSTILE, roles: [] }] });
    await page.goto('/admin/users');
    await expect(rows(page)).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('admin - user editor', () => {
  const editor = `/admin/users/${ID.bob}`;
  const username = (page) => page.locator('#admin-user-username');
  const save = (page) => page.locator('#admin-user-save');
  const status = (page) => page.locator('#admin-user-status');

  test('shows the username, roles, linked accounts and effective permissions', async ({ page }) => {
    await signIn(page);
    await page.goto(editor);
    await expect(page.locator('#admin-user-title')).toHaveText('bob');
    await expect(page.locator('#admin-user-id')).toHaveText(ID.bob);
    await expect(username(page)).toHaveValue('bob');
    const boxes = page.locator('#admin-user-roles input');
    await expect(boxes).toHaveCount(3);
    await expect(boxes.nth(0)).not.toBeChecked();
    await expect(boxes.nth(2)).toBeChecked();
    await expect(boxes.nth(2)).toHaveAttribute('value', ID.bee);
    await expect(page.locator('#admin-user-links li')).toHaveText([/^discord\s*bob#1234$/, /^steam\s*76561198000000000$/]);
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin', 'ratelimit:100']);
  });

  test('saving a new username sends only the username, says Saved in the status line and keeps focus on Save', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await username(page).fill('  robert  ');
    await save(page).click();
    await expect(status(page)).toHaveText('Saved');
    await expect(page.locator('#admin-user-title')).toHaveText('robert');
    await expect(username(page)).toHaveValue('robert');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
    await expect(save(page)).toBeFocused();
    await expect(status(page)).toHaveAttribute('role', 'status');
  });

  test('saving roles sends only the roles', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await page.locator(`#admin-user-roles input[value="${ID.owner}"]`).check();
    await save(page).click();
    await expect(status(page)).toHaveText('Saved');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [ID.owner, ID.bee] } }]);
    await expect(page.locator(`#admin-user-roles input[value="${ID.owner}"]`)).toBeChecked();
  });

  test('saving without a change sends nothing', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await save(page).click();
    await expect(status(page)).toHaveText('Nothing to save');
    expect(await writes()).toEqual([]);
  });

  test('an emptied username is refused in the banner without a request', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await username(page).fill('   ');
    await save(page).click();
    await expect(error(page)).toHaveText('Enter a username');
    await expect(username(page)).toHaveValue('   ');
    expect(await writes()).toEqual([]);
  });

  test("a stale form does not revert another admin's role change", async ({ page }) => {
    const { writes, api } = await signIn(page);
    await page.goto(editor);
    await expect(username(page)).toHaveValue('bob');
    expect((await api('PUT', `/users/${ID.bob}`, { roles: [ID.bee, ID.owner] })).ok()).toBe(true);

    await username(page).fill('robert');
    await save(page).click();
    await expect(status(page)).toHaveText('Saved');
    expect(await writes()).toEqual([
      { method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [ID.bee, ID.owner] } },
      { method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } },
    ]);
    await expect(page.locator(`#admin-user-roles input[value="${ID.owner}"]`)).toBeChecked();
  });

  test('a refused save shows the API message, keeps what was typed, and a good retry clears the message', async ({ page }) => {
    await signIn(page);
    await page.goto(editor);
    await username(page).fill('alice');
    await save(page).click();
    await expect(error(page)).toHaveText('An account with this username already exists');
    await expect(username(page)).toHaveValue('alice');
    await expect(status(page)).toHaveText('');

    await username(page).fill('carol');
    await save(page).click();
    await expect(status(page)).toHaveText('Saved');
    await expect(error(page)).toHaveText('');
  });

  test('without roles.admin the roles are listed by ID and left out of the save', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['users.admin'] });
    await page.goto(editor);
    await expect(page.locator('#admin-user-roles li')).toHaveText([ID.bee]);
    await expect(page.locator('#admin-user-roles input')).toHaveCount(0);
    await expect(page.locator('#admin-user-roles-note')).toBeVisible();
    await username(page).fill('robert');
    await save(page).click();
    await expect(status(page)).toHaveText('Saved');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
  });

  test('a refused links lookup is an error beside its section, not an empty list', async ({ page }) => {
    await signIn(page, { failures: { [`GET /users/${ID.bob}/links`]: { status: 403, detail: 'links are off limits' } } });
    await page.goto(editor);
    await expect(page.locator('#admin-user-links-error')).toHaveText('links are off limits');
    await expect(page.locator('#admin-user-links-empty')).toHaveCount(0);
    await expect(page.locator('#admin-user-form')).toBeVisible();
  });

  test('a failed permissions refresh after a save is reported beside the list, without a plain Saved', async ({ page }) => {
    await signIn(page, { failures: { [`GET /users/${ID.bob}/permissions`]: { status: 500, detail: 'permissions are down', skip: 1 } } });
    await page.goto(editor);
    await username(page).fill('robert');
    await save(page).click();
    await expect(page.locator('#admin-user-permissions-error')).toHaveText('permissions are down');
    await expect(status(page)).toHaveText('Saved, but the permissions below are out of date');
  });

  test('an account with no links or permissions shows the empty states', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/users/${ID.anon}`);
    await expect(page.locator('#admin-user-links-empty')).toBeVisible();
    await expect(page.locator('#admin-user-permissions-empty')).toBeVisible();
    await expect(page.locator('#admin-user-title')).toHaveText('No username');
  });

  test('an unknown user shows the API message and nothing from the URL', async ({ page }) => {
    await signIn(page);
    const res = await page.goto('/admin/users/999');
    expect(res.status()).toBe(404);
    await expect(error(page)).toHaveText('User not found');
    await expect(page.locator('#admin-user-form')).toHaveCount(0);
    await expect(page.locator('body')).not.toContainText('999');
  });

  test('API text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, {
      users: [{ user_id: ID.bob, username: HOSTILE, roles: [ID.bee] }],
      roles: [{ id: ID.bee, name: HOSTILE, description: HOSTILE, grants: [] }],
      links: { [ID.bob]: [{ platform: HOSTILE, platform_username: HOSTILE, platform_id: HOSTILE }] },
    });
    await page.goto(editor);
    await expect(page.locator('#admin-user-title')).toContainText(HOSTILE);
    await expect(page.locator('#admin-user-links')).toContainText(HOSTILE);
    await expect(page.locator('#admin-user-roles')).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('admin - roles', () => {
  const granted = (page) => page.locator('#admin-role-permissions li');
  const heading = (page) => page.locator('#admin-role-permissions-title');
  const editor = `/admin/roles/${ID.bee}`;

  test('the list shows each role with its permissions and a link to its editor', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    const rows = page.locator('#admin-roles li');
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(2)).toContainText('bee_admin');
    await expect(rows.nth(2).locator('span.rounded-full')).toHaveText(['beenamegenerator.admin', 'ratelimit: 100']);
    await expect(rows.nth(2).locator('a')).toHaveAttribute('href', editor);
  });

  test('a refused list shows the API message and no create form', async ({ page }) => {
    await signIn(page, { me: ['users.admin'] });
    await page.goto('/admin/roles');
    await expect(error(page)).toHaveText('You do not have permission to manage roles and permissions');
    await expect(page.locator('#admin-role-create-form')).toHaveCount(0);
  });

  test('creating a role opens its editor', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill(' moderator ');
    await page.locator('#admin-role-create-description').fill('Moderates things');
    await page.locator('#admin-role-create-submit').click();
    await expect(page).toHaveURL(/\/admin\/roles\/\d{19}$/);
    await expect(page.locator('#admin-role-name')).toHaveValue('moderator');
    expect(await writes()).toEqual([{ method: 'POST', path: '/roles', body: { name: 'moderator', description: 'Moderates things' } }]);
  });

  test('a refused create shows the API message and stays on the list', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill('system');
    await page.locator('#admin-role-create-submit').click();
    await expect(error(page)).toHaveText('A role with that name already exists');
    await expect(page).toHaveURL(/\/admin\/roles$/);
    await expect(page.locator('#admin-role-create-name')).toHaveValue('system');
  });

  test('the editor shows the role, its grants with values, and the permissions still to grant', async ({ page }) => {
    await signIn(page);
    await page.goto(editor);
    await expect(page.locator('#admin-role-title')).toHaveText('bee_admin');
    await expect(page.locator('#admin-role-id')).toHaveText(ID.bee);
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_admin');
    await expect(granted(page)).toHaveCount(2);
    await expect(granted(page).nth(1).locator('input[type="number"]')).toHaveValue('100');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd', 'datastore.admin']);
  });

  test('saving a new name sends only the name and updates the page in place', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-name').fill(' bee_manager ');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    await expect(page.locator('#admin-role-title')).toHaveText('bee_manager');
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
    await expect(page.locator('#admin-role-save')).toBeFocused();
  });

  test("a stale form does not revert another admin's description change", async ({ page }) => {
    const { writes, api } = await signIn(page);
    await page.goto(editor);
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_admin');
    expect((await api('PATCH', `/roles/${ID.bee}`, { description: 'Changed elsewhere' })).ok()).toBe(true);

    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    expect(await writes()).toEqual([
      { method: 'PATCH', path: `/roles/${ID.bee}`, body: { description: 'Changed elsewhere' } },
      { method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } },
    ]);
    await expect(page.locator('#admin-role-description')).toHaveValue('Changed elsewhere');
  });

  test('saving without a change sends nothing', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Nothing to save');
    expect(await writes()).toEqual([]);
  });

  test('renaming a built-in role is refused with the API message', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/roles/${ID.system}`);
    await page.locator('#admin-role-name').fill('renamed');
    await page.locator('#admin-role-save').click();
    await expect(error(page)).toHaveText('Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin');
    await expect(page.locator('#admin-role-status')).toHaveText('');
  });

  test('changing an int value puts the whole number and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('250');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('250');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
    await expect(heading(page)).toBeFocused();
  });

  test('a fraction is stopped by the number input before any request', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('1.5');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input:invalid')).toHaveCount(1);
    expect(await writes()).toEqual([]);
  });

  for (const bad of ['', '9007199254740993']) {
    test(`an int value of "${bad}" is refused in the banner without a request`, async ({ page }) => {
      const { writes } = await signIn(page);
      await page.goto(editor);
      await granted(page).nth(1).locator('input').fill(bad);
      await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
      await expect(error(page)).toHaveText('Enter a whole number from -9007199254740992 to 9007199254740992');
      expect(await writes()).toEqual([]);
    });
  }

  test('granting a permission without a value puts it bare', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await expect(page.locator('#admin-role-grant-value').locator('input, textarea')).toHaveCount(0);
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd']);
  });

  test('choosing a permission swaps in the input for its type, and granting puts the typed value', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    const select = page.locator('#admin-role-grant-permission');
    await select.selectOption({ label: 'motd' });
    await expect(page.locator('#admin-role-grant-value input[type="text"]')).toBeVisible();
    await select.selectOption({ label: 'petpictures.pets' });
    await expect(page.locator('#admin-role-grant-value textarea')).toBeVisible();
    await page.locator('#admin-role-grant-value textarea').fill('rex\n  fido  \n\nspot');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pPets}`, body: { value: ['rex', 'fido', 'spot'] } }]);
    await expect(granted(page).nth(2).locator('textarea')).toHaveValue('rex\nfido\nspot');
    await expect(page.locator('#admin-role-grant-submit')).toBeFocused();
  });

  test('a list or text value left empty is refused without a request', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'petpictures.pets' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('Enter at least one item');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('Enter a value');
    expect(await writes()).toEqual([]);
  });

  test('removing a permission deletes the grant by its exact ID and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toContainText(['beenamegenerator.admin']);
    await expect(heading(page)).toBeFocused();
  });

  test('a half-typed grant value survives removing another permission', async ({ page }) => {
    await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('half typed');
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission')).toHaveValue(ID.pMotd);
    await expect(page.locator('#admin-role-grant-value input')).toHaveValue('half typed');
  });

  test('a refused removal shows the API message, and the next action clears it', async ({ page }) => {
    await signIn(page, {
      failures: { [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: { status: 409, detail: 'system and owner keep roles.admin', times: 1 } },
    });
    await page.goto(editor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(error(page)).toHaveText('system and owner keep roles.admin');
    await expect(granted(page)).toHaveCount(2);

    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(error(page)).toHaveText('');
  });

  test('a role holding every permission offers nothing more to grant', async ({ page }) => {
    await signIn(page, {
      roles: [
        {
          id: ID.bee,
          name: 'bee_admin',
          description: 'd',
          grants: [{ id: ID.pBee }, { id: ID.pRate, value: 1 }, { id: ID.pPets, value: ['a'] }, { id: ID.pMotd, value: 'x' }, { id: ID.pStore }],
        },
      ],
    });
    await page.goto(editor);
    await expect(page.locator('#admin-role-grant-form')).toHaveCount(0);
    await expect(page.locator('#admin-role-grant-empty')).toBeVisible();
  });

  test('deleting a role asks first, then deletes it and returns to the list', async ({ page }) => {
    const { writes } = await signIn(page, { users: [] });
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto(editor);
    await page.locator('#admin-role-delete').click();
    await expect.poll(() => messages.length).toBe(1);
    expect(await writes()).toEqual([]);

    await page.locator('#admin-role-delete').click();
    await expect(page).toHaveURL(/\/admin\/roles$/);
    expect(messages).toEqual(['Delete the role bee_admin?', 'Delete the role bee_admin?']);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}`, body: null }]);
  });

  test('deleting a role an account holds shows the API message', async ({ page }) => {
    await signIn(page);
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto(editor);
    await page.locator('#admin-role-delete').click();
    await expect(error(page)).toHaveText('The role is assigned to an account');
    await expect(page).toHaveURL(new RegExp(`/admin/roles/${ID.bee}$`));
  });

  test('an unknown role shows the API message and nothing from the URL', async ({ page }) => {
    await signIn(page);
    const res = await page.goto('/admin/roles/404');
    expect(res.status()).toBe(404);
    await expect(error(page)).toHaveText('Role not found');
    await expect(page.locator('#admin-role-form')).toHaveCount(0);
  });

  test('API text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, {
      permissions: [
        { id: ID.pBee, node: HOSTILE, description: HOSTILE },
        { id: ID.pMotd, node: 'motd', description: HOSTILE, value_type: 'string', merge: 'first' },
        { id: ID.pStore, node: HOSTILE + 'x', description: 'd' },
      ],
      roles: [{ id: ID.bee, name: HOSTILE, description: HOSTILE, grants: [{ id: ID.pBee }, { id: ID.pMotd, value: HOSTILE }] }],
    });
    await page.goto(editor);
    await expect(page.locator('#admin-role-title')).toContainText(HOSTILE);
    await expect(granted(page).nth(0)).toContainText(HOSTILE);
    await expect(granted(page).nth(1).locator('input')).toHaveValue(HOSTILE);
    await expectNoInjection(page);
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-roles li').first()).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('admin - permissions', () => {
  const rows = (page) => page.locator('#admin-permissions li');

  test('lists the permissions with their value type and merge rule', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await expect(rows(page)).toHaveCount(5);
    await expect(rows(page).nth(0).locator('span.rounded-full')).toHaveCount(0);
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText('int, merge max');
    await expect(rows(page).nth(2).locator('span.rounded-full')).toHaveText('string_list, merge union');
  });

  test('a refused list shows the API message and no create form', async ({ page }) => {
    await signIn(page, { me: ['users.admin'] });
    await page.goto('/admin/permissions');
    await expect(error(page)).toHaveText('You do not have permission to manage roles and permissions');
    await expect(page.locator('#admin-permission-create-form')).toHaveCount(0);
  });

  test('creating a permission without a value adds it and resets the form', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill(' pets.write ');
    await page.locator('#admin-permission-create-description').fill('Write pets');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('');
    expect(await writes()).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'pets.write', description: 'Write pets' } }]);
  });

  test('the merge rule only appears for a whole-number permission, and is sent with it', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto('/admin/permissions');
    const merge = page.locator('#admin-permission-create-merge-field');
    await expect(merge).toBeHidden();
    await page.locator('#admin-permission-create-type').selectOption('string');
    await expect(merge).toBeHidden();
    await page.locator('#admin-permission-create-type').selectOption('int');
    await expect(merge).toBeVisible();
    await page.locator('#admin-permission-create-node').fill('quota');
    await page.locator('#admin-permission-create-merge').selectOption('min');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(await writes()).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'quota', description: '', value_type: 'int', merge: 'min' } }]);
  });

  test('a refused create shows the API message and keeps the form', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill('Bad Node');
    await page.locator('#admin-permission-create-submit').click();
    await expect(error(page)).toContainText('Nodes are lower-case words');
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('Bad Node');
  });

  test('deleting a permission asks first, deletes it by its exact ID and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto('/admin/permissions');
    await rows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect.poll(() => messages.length).toBe(1);
    expect(await writes()).toEqual([]);

    await rows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect(rows(page)).toHaveCount(4);
    expect(messages).toEqual(['Delete the permission datastore.admin?', 'Delete the permission datastore.admin?']);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-permissions-title')).toBeFocused();
  });

  test('deleting a permission a role grants shows the API message', async ({ page }) => {
    await signIn(page);
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/admin/permissions');
    await rows(page).nth(0).getByRole('button', { name: 'Delete' }).click();
    await expect(error(page)).toHaveText('The permission is granted by a role');
    await expect(rows(page)).toHaveCount(5);
  });

  test('API text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, { permissions: [{ id: ID.pRate, node: HOSTILE, description: HOSTILE, value_type: 'int', merge: HOSTILE }] });
    await page.goto('/admin/permissions');
    await expect(rows(page).first()).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('admin - changes made while other edits are open', () => {
  const roleEditor = `/admin/roles/${ID.bee}`;
  const grantedRows = (page) => page.locator('#admin-role-permissions li');
  const rateField = (page) => grantedRows(page).nth(1).locator('input');

  test('renaming a role keeps an unsaved value and updates the title and the delete prompt', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await rateField(page).fill('555');
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    await expect(page.locator('#admin-role-title')).toHaveText('bee_manager');
    await expect(rateField(page)).toHaveValue('555');
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);

    const prompts = [];
    page.once('dialog', (dialog) => {
      prompts.push(dialog.message());
      return dialog.dismiss();
    });
    await page.locator('#admin-role-delete').click();
    await expect.poll(() => prompts).toEqual(['Delete the role bee_manager?']);
  });

  test('granting a permission keeps an unsaved value in another row and empties the grant form', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await rateField(page).fill('555');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('hello');
    await page.locator('#admin-role-grant-submit').click();
    await expect(grantedRows(page)).toHaveCount(3);
    await expect(rateField(page)).toHaveValue('555');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'datastore.admin']);
    await expect(page.locator('#admin-role-grant-value textarea')).toHaveValue('');
  });

  test('saving one value keeps the unsaved value in another row', async ({ page }) => {
    const { writes } = await signIn(page, {
      roles: [{ id: ID.bee, name: 'bee_admin', description: 'd', grants: [{ id: ID.pRate, value: 100 }, { id: ID.pMotd, value: 'hi' }] }],
    });
    await page.goto(roleEditor);
    await grantedRows(page).nth(0).locator('input').fill('250');
    await grantedRows(page).nth(1).locator('input').fill('half typed');
    await grantedRows(page).nth(0).getByRole('button', { name: 'Save value of ratelimit' }).click();
    await expect.poll(writes).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
    await expect(grantedRows(page).nth(0).locator('input')).toHaveValue('250');
    await expect(grantedRows(page).nth(1).locator('input')).toHaveValue('half typed');
  });

  test('repeated clicks while a save is in flight send one write', async ({ page }) => {
    const { writes } = await signIn(page, { delays: { [`PATCH /roles/${ID.bee}`]: 600 } });
    await page.goto(roleEditor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click({ clickCount: 3 });
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
  });

  test('a second action while another is in flight is dropped', async ({ page }) => {
    const { writes } = await signIn(page, { delays: { [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: 600 } });
    await page.goto(roleEditor);
    await grantedRows(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await grantedRows(page).nth(1).getByRole('button', { name: 'Remove ratelimit' }).click();
    await expect(grantedRows(page)).toHaveCount(1);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
  });

  test('granting an int value puts the whole number once its permission is free', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await grantedRows(page).nth(1).getByRole('button', { name: 'Remove ratelimit' }).click();
    await expect(grantedRows(page)).toHaveCount(1);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'ratelimit' });
    await page.locator('#admin-role-grant-value input').fill('-7');
    await page.locator('#admin-role-grant-submit').click();
    await expect(grantedRows(page)).toHaveCount(2);
    expect((await writes()).at(-1)).toEqual({ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: -7 } });
  });

  test('the last grant moves focus to the list heading, the others keep it on the button', async ({ page }) => {
    await signIn(page, {
      roles: [{ id: ID.bee, name: 'bee_admin', description: 'd', grants: [{ id: ID.pRate, value: 1 }, { id: ID.pPets, value: ['a'] }, { id: ID.pMotd, value: 'x' }, { id: ID.pStore }] }],
    });
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['beenamegenerator.admin']);
    await page.locator('#admin-role-grant-submit').click();
    await expect(page.locator('#admin-role-grant-empty')).toBeVisible();
    await expect(page.locator('#admin-role-permissions-title')).toBeFocused();
  });

  test('a grant that leaves permissions to grant keeps focus on the Grant button', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(grantedRows(page)).toHaveCount(3);
    await expect(page.locator('#admin-role-grant-submit')).toBeFocused();
  });

  test('an interrupted grant shows the banner and leaves the editor as it was', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await page.route('**/admin/roles/*/permissions', (route) => route.abort('failed'));
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(grantedRows(page)).toHaveCount(2);
    expect(await writes()).toEqual([]);
  });

  test('an interrupted rename shows the banner', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.route(`**/admin/roles/${ID.bee}`, (route) => (route.request().method() === 'POST' ? route.abort('failed') : route.continue()));
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_manager');
  });

  test('an interrupted permission create shows the banner and keeps what was typed', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await page.route('**/admin/permissions', (route) => (route.request().method() === 'POST' ? route.abort('failed') : route.continue()));
    await page.locator('#admin-permission-create-node').fill('pets.write');
    await page.locator('#admin-permission-create-submit').click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('pets.write');
  });

  test('a user whose role has no checkbox keeps it when only the username changes', async ({ page }) => {
    const ghost = '4242424242424242424';
    const { writes } = await signIn(page, { users: [{ user_id: ID.bob, username: 'bob', roles: [ID.bee, ghost] }] });
    await page.goto(`/admin/users/${ID.bob}`);
    await page.locator('#admin-user-username').fill('robert');
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
  });

  test('a failed save empties the status line of the save before it', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/users/${ID.bob}`);
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Nothing to save');
    await page.locator('#admin-user-username').fill('alice');
    await page.locator('#admin-user-save').click();
    await expect(error(page)).toHaveText('An account with this username already exists');
    await expect(page.locator('#admin-user-status')).toHaveText('');
  });

  test('a role change empties the status line of the rename before it', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    await grantedRows(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(grantedRows(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-status')).toHaveText('');
  });

  test('Load more moves focus to the first new user', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users > li:has(> a)')).toHaveCount(250);
    await expect(page.locator('#admin-users > li:has(> a)').nth(200).locator('a')).toBeFocused();
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
