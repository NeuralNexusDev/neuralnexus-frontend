import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;
const API_PATH = new URL(API).pathname;

// Snowflake-sized IDs, above 2^53, so any number conversion changes them.
const ID = {
  system: '354102516314675761',
  bee: '354102516314675763',
  pBee: '354102516314675771',
  pRate: '354102516314675773',
  pPets: '354102516314675778',
  pMotd: '354102516314675779',
  pStore: '354102516314675780',
  alice: '354102516314675901',
  bob: '354102516314675902',
  anon: '354102516314675903',
  newRole: '354102516314675999',
  newPermission: '354102516314676000',
};

const HOSTILE = '<img src=x onerror="window.__xss=1">';

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });
const problem = (status, detail) => ({
  status,
  contentType: 'application/problem+json',
  body: JSON.stringify({ title: 'x', status, detail }),
});

const PERMS = [
  { id: ID.pBee, node: 'beenamegenerator.admin', description: 'Bee name generator' },
  { id: ID.pRate, node: 'ratelimit', description: 'Rate limit', value_type: 'int', merge: 'max' },
  { id: ID.pPets, node: 'petpictures.pets', description: 'Pet pictures', value_type: 'string_list', merge: 'union' },
  { id: ID.pMotd, node: 'motd', description: 'Message of the day', value_type: 'string', merge: 'first' },
  { id: ID.pStore, node: 'datastore.admin', description: 'Data store' },
];

const LINKS = [
  { platform: 'discord', platform_username: 'bob#1234', platform_id: '9', verified: true, login_enabled: true },
  { platform: 'steam', platform_id: '76561198000000000', verified: true, login_enabled: false },
];

function adminState() {
  return {
    me: ['users.admin', 'roles.admin'],
    permissions: structuredClone(PERMS),
    roles: [
      { id: ID.system, name: 'system', description: 'System', permissions: [] },
      { id: ID.bee, name: 'bee_admin', description: 'Bee Name Generator Admin', permissions: [{ ...PERMS[0] }, { ...PERMS[1], value: 100 }] },
    ],
    users: [
      { user_id: ID.alice, username: 'alice', roles: [ID.system] },
      { user_id: ID.bob, username: 'bob', roles: [ID.bee, ID.system] },
      { user_id: ID.anon, username: '', roles: [] },
    ],
    links: { [ID.bob]: LINKS },
    effective: ['beenamegenerator.admin', 'ratelimit:100'],
    effectiveAfterSave: undefined,
    calls: [],
  };
}

/** Bob holds only bee_admin here, so the editor's checkboxes start as system unticked and bee_admin ticked. */
function editorState() {
  const state = adminState();
  state.users[1].roles = [ID.bee];
  return state;
}

const manyUsers = (count) =>
  Array.from({ length: count }, (_, i) => ({ user_id: String(354102516314670000n + BigInt(i + 1)), username: `user${i + 1}`, roles: [] }));

/**
 * Serves the admin endpoints from `state`, recording every call. `overrides` maps "METHOD /path" to a response, or to a
 * function of the call's index that returns one (or undefined to fall through to the default).
 */
async function mockAdmin(page, state, overrides = {}) {
  const counts = {};
  await page.route(/\/api\/v1\/(users|roles|permissions)(\/[^?]*)?(\?.*)?$/, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.slice(API_PATH.length);
    const method = request.method();
    const body = request.postData() ? request.postDataJSON() : null;
    state.calls.push({ method, path, search: url.search, body });
    const key = `${method} ${path}`;
    counts[key] = (counts[key] || 0) + 1;
    let override = overrides[key];
    if (typeof override === 'function') {
      override = override(counts[key]);
    }
    if (override) {
      return route.fulfill(override);
    }

    const parts = path
      .split('/')
      .filter(Boolean)
      .map((part) => decodeURIComponent(part));
    const [kind, id, sub, subId] = parts;

    if (path === '/users/me/permissions') {
      return route.fulfill(json(state.me));
    }
    if (kind === 'users') {
      if (parts.length === 1) {
        const limit = Number(url.searchParams.get('limit'));
        const offset = Number(url.searchParams.get('offset'));
        return route.fulfill(json(state.users.slice(offset, offset + limit)));
      }
      const user = state.users.find((candidate) => candidate.user_id === id);
      if (!user) {
        return route.fulfill(problem(404, 'User not found'));
      }
      if (sub === 'links') {
        return route.fulfill(json(state.links[id] || []));
      }
      if (sub === 'permissions') {
        return route.fulfill(json(state.effective));
      }
      if (method === 'PUT') {
        Object.assign(user, body);
        if (state.effectiveAfterSave) {
          state.effective = state.effectiveAfterSave;
        }
      }
      return route.fulfill(json(user));
    }
    if (kind === 'permissions') {
      if (method === 'GET' && parts.length === 1) {
        return route.fulfill(json(state.permissions));
      }
      if (method === 'POST') {
        const created = { id: ID.newPermission, ...body, description: body.description || '' };
        state.permissions.push(created);
        return route.fulfill(json(created, 201));
      }
      state.permissions = state.permissions.filter((candidate) => candidate.id !== id);
      return route.fulfill({ status: 204 });
    }
    if (method === 'GET' && parts.length === 1) {
      return route.fulfill(json(state.roles));
    }
    if (method === 'POST') {
      const created = { id: ID.newRole, name: body.name, description: body.description || '', permissions: [] };
      state.roles.push(created);
      return route.fulfill(json(created, 201));
    }
    const role = state.roles.find((candidate) => candidate.id === id);
    if (!role) {
      return route.fulfill(problem(404, 'Role not found'));
    }
    if (parts.length === 2) {
      if (method === 'PATCH') {
        Object.assign(role, body);
      }
      if (method === 'DELETE') {
        state.roles = state.roles.filter((candidate) => candidate !== role);
        return route.fulfill({ status: 204 });
      }
      return route.fulfill(json(role));
    }
    const permission = state.permissions.find((candidate) => candidate.id === subId);
    const index = role.permissions.findIndex((candidate) => candidate.id === permission.id);
    if (method === 'PUT') {
      const granted = body ? { ...permission, value: body.value } : { ...permission };
      if (index >= 0) {
        role.permissions[index] = granted;
      } else {
        role.permissions.push(granted);
      }
    } else if (index >= 0) {
      role.permissions.splice(index, 1);
    }
    return route.fulfill({ status: 204 });
  });
}

const writes = (state) => state.calls.filter((call) => call.method !== 'GET').map(({ method, path, body }) => ({ method, path, body }));

/** Lets the page's promise callbacks run after a response, so a hidden check cannot pass before the page reacted. */
const settle = (page) => page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));

async function expectNoInjection(page) {
  await expect(page.locator('img[src="x"]')).toHaveCount(0);
  expect(await page.evaluate(() => window.__xss)).toBeUndefined();
}

async function mockAccountPage(page, permissions) {
  await page.route(`${API}/users/me`, (route) => route.fulfill(json({ username: 'admin' })));
  await page.route(`${API}/users/me/links`, (route) => route.fulfill(json([])));
  await page.route(`${API}/users/me/settings`, (route) => route.fulfill(json({ password_auth: true })));
  await page.route(`${API}/users/me/permissions`, (route) => route.fulfill(json(permissions)));
}

async function openAccount(page, permissions) {
  await mockAccountPage(page, permissions);
  const answered = page.waitForResponse(`${API}/users/me/permissions`);
  await page.goto('/account');
  await answered;
  await settle(page);
}

test.describe('admin - settings link', () => {
  for (const permissions of [['users.admin'], ['roles.admin'], ['ratelimit:1000', 'users.admin'], ['roles.admin:1']]) {
    test(`the account page links to the admin dashboard with ${permissions.join(', ')}`, async ({ page }) => {
      await openAccount(page, permissions);
      const link = page.locator('#admin-dashboard-link');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute('href', '/admin');
    });
  }

  for (const permissions of [[], ['ratelimit:1000'], ['beenamegenerator.admin'], ['users.administrator'], ['xusers.admin'], ['roles.adminx:1']]) {
    test(`the account page has no admin link with [${permissions.join(', ')}]`, async ({ page }) => {
      await openAccount(page, permissions);
      await expect(page.locator('#account-username')).toHaveText('admin');
      await expect(page.locator('#admin-dashboard-link')).toBeHidden();
    });
  }
});

test.describe('admin - dashboard', () => {
  test('users.admin shows the users card', async ({ page }) => {
    await mockAdmin(page, { ...adminState(), me: ['users.admin'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-users-link')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toHaveAttribute('href', '/admin/users');
    await expect(page.locator('#admin-roles-link')).toBeHidden();
    await expect(page.locator('#admin-permissions-link')).toBeHidden();
    await expect(page.locator('#admin-denied')).toBeHidden();
  });

  test('roles.admin shows the roles and permissions cards', async ({ page }) => {
    await mockAdmin(page, { ...adminState(), me: ['roles.admin'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-roles-link')).toHaveAttribute('href', '/admin/roles');
    await expect(page.locator('#admin-permissions-link')).toHaveAttribute('href', '/admin/permissions');
    await expect(page.locator('#admin-users-link')).toBeHidden();
    await expect(page.locator('#admin-denied')).toBeHidden();
  });

  test('an account without admin permissions sees that it has none', async ({ page }) => {
    await mockAdmin(page, { ...adminState(), me: ['ratelimit:1000'] });
    await page.goto('/admin');
    await expect(page.locator('#admin-denied')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toBeHidden();
    await expect(page.locator('#admin-roles-link')).toBeHidden();
    await expect(page.locator('#admin-permissions-link')).toBeHidden();
  });

  test('a failed permissions lookup shows the API message', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /users/me/permissions': problem(500, 'no access') });
    await page.goto('/admin');
    await expect(page.locator('#admin-error')).toHaveText('no access');
    await expect(page.locator('#admin-users-link')).toBeHidden();
  });

  test('a network failure shows a generic message', async ({ page }) => {
    await page.route(`${API}/users/me/permissions`, (route) => route.abort('failed'));
    await page.goto('/admin');
    await expect(page.locator('#admin-error')).toHaveText('Failed to load your permissions');
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await page.route(`${API}/users/me/permissions`, (route) => route.fulfill(problem(401, 'sign in')));
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin');
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('admin - user list', () => {
  const rows = (page) => page.locator('#admin-users li');

  test('lists every user with a link to its editor and role names', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(3);
    await expect(rows(page).nth(0)).toContainText('alice');
    await expect(rows(page).nth(0)).toContainText(ID.alice);
    await expect(rows(page).nth(0).locator('a')).toHaveAttribute('href', `/admin/users/${ID.alice}`);
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText(['bee_admin', 'system']);
    await expect(rows(page).nth(2)).toContainText('No username');
    await expect(page.locator('#admin-users-empty')).toBeHidden();
  });

  test('the search box filters by username or user ID, ignoring case and surrounding spaces', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(3);

    await page.locator('#admin-users-search').fill('  BO ');
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('bob');

    await page.locator('#admin-users-search').fill('675903');
    await expect(rows(page)).toHaveCount(1);
    await expect(rows(page)).toContainText('No username');

    await page.locator('#admin-users-search').fill('nobody');
    await expect(rows(page)).toHaveCount(0);
    await expect(page.locator('#admin-users-empty')).toBeVisible();
  });

  test('role IDs are shown when the caller cannot list roles', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /roles': problem(403, 'forbidden') });
    await page.goto('/admin/users');
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText([ID.bee, ID.system]);
  });

  test('a failed roles lookup is an error, not a missing permission', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /roles': problem(500, 'roles are down') });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-error')).toHaveText('roles are down');
    await expect(page.locator('#admin-users-tools')).toBeHidden();
  });

  test('a refused list shows the API message and no search box', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /users': problem(403, 'You do not have permission to list users') });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-error')).toHaveText('You do not have permission to list users');
    await expect(rows(page)).toHaveCount(0);
    await expect(page.locator('#admin-users-search')).toBeHidden();
  });

  test('a network failure shows a generic message', async ({ page }) => {
    await page.route(/\/api\/v1\/users(\?.*)?$/, (route) => route.abort('failed'));
    await page.route(`${API}/roles`, (route) => route.fulfill(json([])));
    await page.goto('/admin/users');
    await expect(page.locator('#admin-error')).toHaveText('Failed to load users');
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /users': problem(401, 'sign in') });
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/users');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('loads a page at a time and offers more only while a page comes back full', async ({ page }) => {
    const state = { ...adminState(), users: manyUsers(250) };
    await mockAdmin(page, state);
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await expect(page.locator('#admin-users-more')).toBeVisible();

    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(250);
    await expect(page.locator('#admin-users-more')).toBeHidden();
    expect(state.calls.filter((call) => call.path === '/users').map((call) => call.search)).toEqual(['?limit=200&offset=0', '?limit=200&offset=200']);
    await expect(rows(page).last()).toContainText('user250');
  });

  for (const lastPage of [199, 150, 1]) {
    test(`a last page of ${lastPage} users ends the list`, async ({ page }) => {
      await mockAdmin(page, { ...adminState(), users: manyUsers(200 + lastPage) });
      await page.goto('/admin/users');
      await page.locator('#admin-users-more-button').click();
      await expect(rows(page)).toHaveCount(200 + lastPage);
      await expect(page.locator('#admin-users-more')).toBeHidden();
    });
  }

  test('a list of exactly one full page still offers more, and an empty next page ends it', async ({ page }) => {
    const state = { ...adminState(), users: manyUsers(200) };
    await mockAdmin(page, state);
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await expect(page.locator('#admin-users-more')).toBeVisible();
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users-more')).toBeHidden();
    await expect(rows(page)).toHaveCount(200);
    expect(state.calls.filter((call) => call.path === '/users')).toHaveLength(2);
  });

  test('a short first page offers no more', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(3);
    await expect(page.locator('#admin-users-more')).toBeHidden();
  });

  test('search covers the users loaded after a Load more', async ({ page }) => {
    await mockAdmin(page, { ...adminState(), users: manyUsers(250) });
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await page.locator('#admin-users-search').fill('user250');
    await expect(rows(page)).toHaveCount(0);
    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(1);
  });

  test('a failed Load more shows the API message, keeps the list, and a retry clears the message', async ({ page }) => {
    await mockAdmin(page, { ...adminState(), users: manyUsers(250) }, { 'GET /users': (call) => (call === 2 ? problem(500, 'Failed to list users') : undefined) });
    await page.goto('/admin/users');
    await expect(rows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-error')).toHaveText('Failed to list users');
    await expect(rows(page)).toHaveCount(200);
    await expect(page.locator('#admin-users-more-button')).toBeEnabled();

    await page.locator('#admin-users-more-button').click();
    await expect(rows(page)).toHaveCount(250);
    await expect(page.locator('#admin-error')).toBeHidden();
  });
});

test.describe('admin - user editor', () => {
  const editor = `/admin/users/${ID.bob}`;
  const username = (page) => page.locator('#admin-user-username');
  const saveButton = (page) => page.locator('#admin-user-save');

  test('shows the username, roles, linked accounts and effective permissions', async ({ page }) => {
    await mockAdmin(page, editorState());
    await page.goto(editor);
    await expect(page.locator('#admin-user-title')).toHaveText('bob');
    await expect(page.locator('#admin-user-id')).toHaveText(ID.bob);
    await expect(username(page)).toHaveValue('bob');
    const boxes = page.locator('#admin-user-roles input');
    await expect(boxes).toHaveCount(2);
    await expect(boxes.nth(0)).not.toBeChecked();
    await expect(boxes.nth(1)).toBeChecked();
    await expect(boxes.nth(0)).toHaveAttribute('value', ID.system);
    await expect(boxes.nth(1)).toHaveAttribute('value', ID.bee);
    await expect(page.locator('#admin-user-roles li').nth(1)).toContainText('Bee Name Generator Admin');
    await expect(page.locator('#admin-user-links li')).toHaveText(['discordbob#1234', 'steam76561198000000000']);
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin', 'ratelimit:100']);
    await expect(page.locator('#admin-user-roles-note')).toBeHidden();
    await expect(page.locator('#admin-user-status')).not.toHaveAttribute('hidden', /.*/);
    await expect(page.locator('#admin-user-links-empty')).toBeHidden();
    await expect(page.locator('#admin-user-permissions-empty')).toBeHidden();
  });

  test('an account with no links or permissions shows the empty states', async ({ page }) => {
    await mockAdmin(page, { ...editorState(), links: {}, effective: [] });
    await page.goto(editor);
    await expect(page.locator('#admin-user-links-empty')).toBeVisible();
    await expect(page.locator('#admin-user-permissions-empty')).toBeVisible();
  });

  test('editing only the username sends only the username', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(page.locator('#admin-user-form')).toBeVisible();
    await username(page).fill('  robert  ');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
    await expect(page.locator('#admin-user-title')).toHaveText('robert');
  });

  test('editing only the roles sends only the roles, as exact IDs', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-user-roles input').nth(0).check();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [ID.system, ID.bee] } }]);
  });

  test('editing both sends both, and the saved roles stay ticked', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await username(page).fill('robert');
    await page.locator('#admin-user-roles input').nth(0).check();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert', roles: [ID.system, ID.bee] } }]);
    await expect(page.locator('#admin-user-roles input').nth(0)).toBeChecked();
    await expect(page.locator('#admin-user-roles input').nth(1)).toBeChecked();
  });

  test('swapping one role for another sends the new list', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-user-roles input').nth(1).uncheck();
    await page.locator('#admin-user-roles input').nth(0).check();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [ID.system] } }]);
  });

  test('unticking every role sends an empty list', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-user-roles input').nth(1).uncheck();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [] } }]);
  });

  test('the Save button keeps focus after a save', async ({ page }) => {
    await mockAdmin(page, editorState());
    await page.goto(editor);
    await username(page).fill('robert');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    await expect(saveButton(page)).toBeFocused();
  });

  test('saving without a change sends nothing', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(page.locator('#admin-user-form')).toBeVisible();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Nothing to save');
    expect(writes(state)).toEqual([]);
  });

  test('an emptied username is refused without a request', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await username(page).fill('   ');
    await saveButton(page).click();
    await expect(page.locator('#admin-error')).toHaveText('Enter a username');
    await expect(page.locator('#admin-user-status')).toHaveText('');
    expect(writes(state)).toEqual([]);
  });

  test('an account without a username can stay that way while its roles change', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto(`/admin/users/${ID.anon}`);
    await expect(page.locator('#admin-user-title')).toHaveText('No username');
    await page.locator('#admin-user-roles input').nth(0).check();
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.anon}`, body: { roles: [ID.system] } }]);
  });

  test('saving refreshes the effective permissions before it says Saved', async ({ page }) => {
    const state = { ...editorState(), effectiveAfterSave: ['beenamegenerator.admin', 'users.admin'] };
    await mockAdmin(page, state);
    await page.goto(editor);
    await username(page).fill('robert');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin', 'users.admin']);
    await expect(saveButton(page)).toBeEnabled();
  });

  test('a failed permissions refresh after a save is reported on its own, without Saved', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state, { [`GET /users/${ID.bob}/permissions`]: (call) => (call === 2 ? problem(500, 'permissions are down') : undefined) });
    await page.goto(editor);
    await username(page).fill('robert');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-permissions-error')).toHaveText('permissions are down');
    await expect(page.locator('#admin-user-status')).toHaveText('Saved, but the permissions below are out of date');
    await expect(page.locator('#admin-error')).toBeHidden();
    await expect(page.locator('#admin-user-permissions-empty')).toBeHidden();
    expect(writes(state)).toHaveLength(1);
  });

  test('a refused save shows the API message, keeps the form editable, and a good retry clears the message', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state, {
      [`PUT /users/${ID.bob}`]: (call) => (call === 1 ? problem(409, 'An account with this username already exists') : undefined),
    });
    await page.goto(editor);
    await username(page).fill('alice');
    await saveButton(page).click();
    await expect(page.locator('#admin-error')).toHaveText('An account with this username already exists');
    await expect(page.locator('#admin-user-status')).toHaveText('');
    await expect(saveButton(page)).toBeEnabled();

    await username(page).fill('carol');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    await expect(page.locator('#admin-error')).toBeHidden();
  });

  test('a network failure on save shows a generic message', async ({ page }) => {
    await mockAdmin(page, editorState(), { [`PUT /users/${ID.bob}`]: undefined });
    await page.goto(editor);
    await page.route(`${API}/users/${ID.bob}`, (route) => (route.request().method() === 'PUT' ? route.abort('failed') : route.fallback()));
    await username(page).fill('robert');
    await saveButton(page).click();
    await expect(page.locator('#admin-error')).toHaveText('Failed to save the user');
    await expect(saveButton(page)).toBeEnabled();
  });

  test('without roles.admin the roles are listed by ID and left out of the save', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state, { 'GET /roles': problem(403, 'forbidden') });
    await page.goto(editor);
    await expect(page.locator('#admin-user-roles li')).toHaveText([ID.bee]);
    await expect(page.locator('#admin-user-roles input')).toHaveCount(0);
    await expect(page.locator('#admin-user-roles-note')).toBeVisible();
    await username(page).fill('robert');
    await saveButton(page).click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
  });

  test('a failed roles lookup is an error, not a missing permission', async ({ page }) => {
    await mockAdmin(page, editorState(), { 'GET /roles': problem(500, 'roles are down') });
    await page.goto(editor);
    await expect(page.locator('#admin-error')).toHaveText('roles are down');
    await expect(page.locator('#admin-user-form')).toBeHidden();
    await expect(page.locator('#admin-user-roles-note')).toBeHidden();
  });

  test('a refused links or permissions lookup is shown as an error, not as an empty list', async ({ page }) => {
    await mockAdmin(page, editorState(), {
      [`GET /users/${ID.bob}/links`]: problem(403, 'links are off limits'),
      [`GET /users/${ID.bob}/permissions`]: problem(500, 'permissions are down'),
    });
    await page.goto(editor);
    await expect(page.locator('#admin-user-links-error')).toHaveText('links are off limits');
    await expect(page.locator('#admin-user-permissions-error')).toHaveText('permissions are down');
    await expect(page.locator('#admin-user-links-empty')).toBeHidden();
    await expect(page.locator('#admin-user-permissions-empty')).toBeHidden();
    await expect(page.locator('#admin-user-form')).toBeVisible();
  });

  test('an unknown user shows the API message and nothing from the URL', async ({ page }) => {
    await mockAdmin(page, editorState());
    await page.goto('/admin/users/999');
    await expect(page.locator('#admin-error')).toHaveText('User not found');
    await expect(page.locator('#admin-user-form')).toBeHidden();
    await expect(page.locator('#admin-user-id')).toHaveText('');
  });

  test('an encoded user ID in the path is decoded for the requests', async ({ page }) => {
    const state = editorState();
    await mockAdmin(page, state);
    await page.goto('/admin/users/a%2Fb');
    await expect(page.locator('#admin-error')).toHaveText('User not found');
    expect(state.calls.map((call) => call.path)).toContain('/users/a%2Fb');
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await mockAdmin(page, editorState(), { [`GET /users/${ID.bob}`]: problem(401, 'sign in') });
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto(editor);
    await expect(page).toHaveURL(/\/login$/);
  });

  test('a network failure on load shows a generic message', async ({ page }) => {
    await page.route(/\/api\/v1\/(users|roles)\/?.*/, (route) => route.abort('failed'));
    await page.goto(editor);
    await expect(page.locator('#admin-error')).toHaveText('Failed to load the user');
    await expect(page.locator('#admin-user-form')).toBeHidden();
  });
});

test.describe('admin - roles list', () => {
  test('lists the roles with their permissions and a link to each editor', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/roles');
    const rows = page.locator('#admin-roles li');
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(1)).toContainText('bee_admin');
    await expect(rows.nth(1)).toContainText('Bee Name Generator Admin');
    await expect(rows.nth(1).locator('span.rounded-full')).toHaveText(['beenamegenerator.admin', 'ratelimit: 100']);
    await expect(rows.nth(1).locator('a')).toHaveAttribute('href', `/admin/roles/${ID.bee}`);
    await expect(page.locator('#admin-roles-empty')).toBeHidden();
  });

  test('a refused list shows the API message and no create form', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /roles': problem(403, 'You do not have permission to manage roles and permissions') });
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-error')).toHaveText('You do not have permission to manage roles and permissions');
    await expect(page.locator('#admin-role-create-form')).toBeHidden();
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /roles': problem(401, 'sign in') });
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/roles');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('creating a role posts it and opens its editor', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill(' moderator ');
    await page.locator('#admin-role-create-description').fill('Moderates things');
    await page.locator('#admin-role-create-submit').click();
    await expect(page).toHaveURL(new RegExp(`/admin/roles/${ID.newRole}$`));
    expect(writes(state)).toEqual([{ method: 'POST', path: '/roles', body: { name: 'moderator', description: 'Moderates things' } }]);
    await expect(page.locator('#admin-role-name')).toHaveValue('moderator');
  });

  test('a refused create shows the API message and stays on the list', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'POST /roles': problem(409, 'A role with that name already exists') });
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill('system');
    await page.locator('#admin-role-create-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('A role with that name already exists');
    await expect(page).toHaveURL(/\/admin\/roles$/);
    await expect(page.locator('#admin-role-create-submit')).toBeEnabled();
  });
});

test.describe('admin - role editor', () => {
  const editor = `/admin/roles/${ID.bee}`;
  const granted = (page) => page.locator('#admin-role-permissions li');
  const heading = (page) => page.locator('#admin-role-permissions-title');
  const error = (page) => page.locator('#admin-error');

  test('shows the role, its granted permissions with values, and the ones still to grant', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(editor);
    await expect(page.locator('#admin-role-title')).toHaveText('bee_admin');
    await expect(page.locator('#admin-role-id')).toHaveText(ID.bee);
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_admin');
    await expect(page.locator('#admin-role-description')).toHaveValue('Bee Name Generator Admin');
    await expect(granted(page)).toHaveCount(2);
    await expect(granted(page).nth(0)).toContainText('beenamegenerator.admin');
    await expect(granted(page).nth(0).locator('input')).toHaveCount(0);
    await expect(granted(page).nth(1).locator('input')).toHaveValue('100');
    await expect(page.locator('#admin-role-status')).not.toHaveAttribute('hidden', /.*/);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd', 'datastore.admin']);
  });

  test('saving a new name sends only the name', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(page.locator('#admin-role-content')).toBeVisible();
    await page.locator('#admin-role-name').fill(' bee_manager ');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
    await expect(page.locator('#admin-role-title')).toHaveText('bee_manager');
  });

  test('saving a new description sends only the description', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-role-description').fill(' Manages bees ');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { description: 'Manages bees' } }]);
  });

  test('saving without a change sends nothing', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(page.locator('#admin-role-content')).toBeVisible();
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Nothing to save');
    expect(writes(state)).toEqual([]);
  });

  test('a refused save shows the API message and no Saved', async ({ page }) => {
    const message = 'Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin';
    await mockAdmin(page, adminState(), { [`PATCH /roles/${ID.system}`]: problem(409, message) });
    await page.goto(`/admin/roles/${ID.system}`);
    await page.locator('#admin-role-name').fill('renamed');
    await page.locator('#admin-role-save').click();
    await expect(error(page)).toHaveText(message);
    await expect(page.locator('#admin-role-status')).toHaveText('');
    await expect(page.locator('#admin-role-save')).toBeEnabled();
  });

  test('a failed reload after a save shows the error without Saved', async ({ page }) => {
    await mockAdmin(page, adminState(), { [`GET /roles/${ID.bee}`]: (call) => (call === 2 ? problem(500, 'roles are down') : undefined) });
    await page.goto(editor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(error(page)).toHaveText('roles are down');
    await expect(page.locator('#admin-role-status')).toHaveText('');
  });

  test('changing an int value puts the whole number against the exact IDs', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('250');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('250');
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
  });

  test('the largest safe integer is accepted', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('9007199254740991');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect.poll(() => writes(state)).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 9007199254740991 } }]);
    await expect(page.locator('#admin-error')).toBeHidden();
  });

  for (const bad of ['', '1.5', '9007199254740992', '9007199254740993', '-9007199254740993', '1e3', '007']) {
    test(`an int value of "${bad}" is refused without a request`, async ({ page }) => {
      const state = adminState();
      await mockAdmin(page, state);
      await page.goto(editor);
      await granted(page).nth(1).locator('input').fill(bad);
      await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
      await expect(error(page)).toHaveText('Enter a whole number from -9007199254740991 to 9007199254740991');
      expect(writes(state)).toEqual([]);
    });
  }

  test('granting a permission without a value puts it bare', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await expect(page.locator('#admin-role-grant-value input, #admin-role-grant-value textarea')).toHaveCount(0);
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd']);
  });

  test('granting a list permission puts the trimmed non-empty lines', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'petpictures.pets' });
    await page.locator('#admin-role-grant-value textarea').fill('rex\n  fido  \n\nspot');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pPets}`, body: { value: ['rex', 'fido', 'spot'] } }]);
    await expect(granted(page).nth(2).locator('textarea')).toHaveValue('rex\nfido\nspot');
  });

  test('granting a text permission puts the trimmed text', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('  hello  ');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pMotd}`, body: { value: 'hello' } }]);
  });

  test('editing an existing text or list grant puts the new value', async ({ page }) => {
    const state = adminState();
    state.roles[1].permissions.push({ ...PERMS[3], value: 'old' }, { ...PERMS[2], value: ['rex'] });
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(granted(page)).toHaveCount(4);
    await granted(page).nth(2).locator('input').fill('  new  ');
    await granted(page).nth(2).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(2).locator('input')).toHaveValue('new');
    await granted(page).nth(3).locator('textarea').fill('rex\nfido');
    await granted(page).nth(3).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(3).locator('textarea')).toHaveValue('rex\nfido');
    expect(writes(state)).toEqual([
      { method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pMotd}`, body: { value: 'new' } },
      { method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pPets}`, body: { value: ['rex', 'fido'] } },
    ]);
  });

  test('a list or text value left empty is refused without a request', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'petpictures.pets' });
    await page.locator('#admin-role-grant-value textarea').fill('  \n ');
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('Enter at least one item');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('Enter a value');
    expect(writes(state)).toEqual([]);
  });

  test('removing a permission deletes the grant by its exact IDs and moves focus to the list heading', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto(editor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    expect(writes(state)).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toContainText(['beenamegenerator.admin']);
    await expect(heading(page)).toBeFocused();
  });

  test('saving a value moves focus to the list heading', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('5');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('5');
    await expect(heading(page)).toBeFocused();
  });

  test('granting keeps focus on the Grant button', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    await expect(page.locator('#admin-role-grant-submit')).toBeFocused();
  });

  test('a half-typed grant value survives removing another permission', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('half typed');
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission')).toHaveValue(ID.pMotd);
    await expect(page.locator('#admin-role-grant-value input')).toHaveValue('half typed');
  });

  test('granting the selected permission clears the grant value and selects the next one', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(editor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('hello');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    await expect(page.locator('#admin-role-grant-permission')).toHaveValue(ID.pPets);
    await expect(page.locator('#admin-role-grant-value textarea')).toHaveValue('');
  });

  test('a refused removal shows the API message, keeps the grant, and the next action clears the message', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state, {
      [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: (call) => (call === 1 ? problem(409, 'system and owner keep roles.admin') : undefined),
    });
    await page.goto(editor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(error(page)).toHaveText('system and owner keep roles.admin');
    await expect(granted(page)).toHaveCount(2);

    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(error(page)).toBeHidden();
  });

  test('a role holding every permission offers nothing more to grant', async ({ page }) => {
    const state = adminState();
    state.roles[1].permissions = state.permissions.map((permission) => ({ ...permission }));
    await mockAdmin(page, state);
    await page.goto(editor);
    await expect(page.locator('#admin-role-grant-form')).toBeHidden();
    await expect(page.locator('#admin-role-grant-empty')).toBeVisible();
  });

  test('a role granting nothing says so', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto(`/admin/roles/${ID.system}`);
    await expect(page.locator('#admin-role-permissions-empty')).toBeVisible();
  });

  test('deleting a role asks first, then deletes it and returns to the list', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto(editor);
    await page.locator('#admin-role-delete').click();
    await expect.poll(() => messages.length).toBe(1);
    expect(writes(state)).toEqual([]);

    await page.locator('#admin-role-delete').click();
    await expect(page).toHaveURL(/\/admin\/roles$/);
    expect(messages).toEqual(['Delete the role bee_admin?', 'Delete the role bee_admin?']);
    expect(writes(state)).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}`, body: null }]);
  });

  test('a refused role delete shows the API message', async ({ page }) => {
    await mockAdmin(page, adminState(), { [`DELETE /roles/${ID.bee}`]: problem(409, 'The role is assigned to an account') });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto(editor);
    await page.locator('#admin-role-delete').click();
    await expect(error(page)).toHaveText('The role is assigned to an account');
    await expect(page).toHaveURL(new RegExp(`/admin/roles/${ID.bee}$`));
    await expect(page.locator('#admin-role-delete')).toBeEnabled();
  });

  test('an unknown role shows the API message, no editor and nothing from the URL', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/roles/404');
    await expect(error(page)).toHaveText('Role not found');
    await expect(page.locator('#admin-role-content')).toBeHidden();
    await expect(page.locator('#admin-role-id')).toHaveText('');
  });

  test('a failed permissions lookup shows the API message and no editor', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /permissions': problem(500, 'permissions are down') });
    await page.goto(editor);
    await expect(error(page)).toHaveText('permissions are down');
    await expect(page.locator('#admin-role-content')).toBeHidden();
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await mockAdmin(page, adminState(), { [`GET /roles/${ID.bee}`]: problem(401, 'sign in') });
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto(editor);
    await expect(page).toHaveURL(/\/login$/);
  });

  test('a network failure on load shows a generic message', async ({ page }) => {
    await page.route(/\/api\/v1\/(roles|permissions)\/?.*/, (route) => route.abort('failed'));
    await page.goto(editor);
    await expect(error(page)).toHaveText('Failed to load the role');
  });
});

test.describe('admin - permissions', () => {
  const rows = (page) => page.locator('#admin-permissions li');

  test('lists the permissions with their value type and merge rule', async ({ page }) => {
    await mockAdmin(page, adminState());
    await page.goto('/admin/permissions');
    await expect(rows(page)).toHaveCount(5);
    await expect(rows(page).nth(0)).toContainText('beenamegenerator.admin');
    await expect(rows(page).nth(0).locator('span.rounded-full')).toHaveCount(0);
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText('int, merge max');
    await expect(rows(page).nth(2).locator('span.rounded-full')).toHaveText('string_list, merge union');
  });

  test('a refused list shows the API message and no create form', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /permissions': problem(403, 'no access') });
    await page.goto('/admin/permissions');
    await expect(page.locator('#admin-error')).toHaveText('no access');
    await expect(page.locator('#admin-permission-create-form')).toBeHidden();
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /permissions': problem(401, 'sign in') });
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin/permissions');
    await expect(page).toHaveURL(/\/login$/);
  });

  test('creating a permission without a value sends only the node and description', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill(' pets.write ');
    await page.locator('#admin-permission-create-description').fill('Write pets');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'pets.write', description: 'Write pets' } }]);
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('');
  });

  test('an int permission asks for the merge rule and sends it', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto('/admin/permissions');
    await expect(page.locator('#admin-permission-create-merge-field')).toBeHidden();
    await page.locator('#admin-permission-create-type').selectOption('int');
    await expect(page.locator('#admin-permission-create-merge-field')).toBeVisible();
    await page.locator('#admin-permission-create-node').fill('quota');
    await page.locator('#admin-permission-create-merge').selectOption('min');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'quota', description: '', value_type: 'int', merge: 'min' } }]);
    await expect(page.locator('#admin-permission-create-merge-field')).toBeHidden();
  });

  test('a text permission sends its value type and no merge rule', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-type').selectOption('string');
    await page.locator('#admin-permission-create-node').fill('greeting');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'greeting', description: '', value_type: 'string' } }]);
  });

  test('a switch back to no value drops the merge rule', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-type').selectOption('int');
    await page.locator('#admin-permission-create-type').selectOption('');
    await page.locator('#admin-permission-create-node').fill('plain');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)[0].body).toEqual({ node: 'plain', description: '' });
  });

  test('a refused create shows the API message and keeps the form', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'POST /permissions': problem(400, 'Nodes are lower-case words') });
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill('Bad Node');
    await page.locator('#admin-permission-create-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('Nodes are lower-case words');
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('Bad Node');
    await expect(page.locator('#admin-permission-create-submit')).toBeEnabled();
  });

  test('deleting a permission asks first, deletes it by its exact ID and moves focus to the list heading', async ({ page }) => {
    const state = adminState();
    await mockAdmin(page, state);
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto('/admin/permissions');
    await rows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect.poll(() => messages.length).toBe(1);
    expect(writes(state)).toEqual([]);

    await rows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect(rows(page)).toHaveCount(4);
    expect(messages).toEqual(['Delete the permission datastore.admin?', 'Delete the permission datastore.admin?']);
    expect(writes(state)).toEqual([{ method: 'DELETE', path: `/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-permissions-title')).toBeFocused();
  });

  test('a refused delete shows the API message and keeps the permission', async ({ page }) => {
    await mockAdmin(page, adminState(), { [`DELETE /permissions/${ID.pBee}`]: problem(409, 'The permission is granted by a role') });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/admin/permissions');
    await rows(page).nth(0).getByRole('button', { name: 'Delete' }).click();
    await expect(page.locator('#admin-error')).toHaveText('The permission is granted by a role');
    await expect(rows(page)).toHaveCount(5);
  });
});

test.describe('admin - API text is never HTML', () => {
  const h = (label) => `${label} ${HOSTILE}`;

  test('the dashboard error', async ({ page }) => {
    await mockAdmin(page, adminState(), { 'GET /users/me/permissions': problem(500, h('detail')) });
    await page.goto('/admin');
    await expect(page.locator('#admin-error')).toContainText(h('detail'));
    await expectNoInjection(page);
  });

  test('the user list', async ({ page }) => {
    const state = adminState();
    state.users = [{ user_id: h('id'), username: h('name'), roles: [ID.bee, h('role')] }];
    state.roles[1].name = h('role name');
    await mockAdmin(page, state);
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toContainText(h('name'));
    await expect(page.locator('#admin-users li')).toContainText(h('id'));
    await expect(page.locator('#admin-users li')).toContainText(h('role name'));
    await expect(page.locator('#admin-users li')).toContainText(h('role'));
    await expectNoInjection(page);
  });

  test('the user list without role names', async ({ page }) => {
    const state = adminState();
    state.users = [{ user_id: ID.bob, username: 'bob', roles: [h('role')] }];
    await mockAdmin(page, state, { 'GET /roles': problem(403, 'forbidden') });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toContainText(h('role'));
    await expectNoInjection(page);
  });

  test('the user editor', async ({ page }) => {
    const state = adminState();
    state.users[1] = { user_id: ID.bob, username: h('name'), roles: [ID.bee] };
    state.roles[1].name = h('role name');
    state.roles[1].description = h('role description');
    state.links[ID.bob] = [{ platform: h('platform'), platform_username: h('platform user'), platform_id: h('platform id') }];
    state.effective = [h('permission')];
    await mockAdmin(page, state);
    await page.goto(`/admin/users/${ID.bob}`);
    await expect(page.locator('#admin-user-title')).toContainText(h('name'));
    await expect(page.locator('#admin-user-roles')).toContainText(h('role name'));
    await expect(page.locator('#admin-user-roles')).toContainText(h('role description'));
    await expect(page.locator('#admin-user-links')).toContainText(h('platform'));
    await expect(page.locator('#admin-user-links')).toContainText(h('platform user'));
    await expect(page.locator('#admin-user-permissions')).toContainText(h('permission'));
    await expectNoInjection(page);
  });

  test('the user editor with a hostile ID, read-only roles and refused sections', async ({ page }) => {
    const state = adminState();
    const id = h('id');
    state.users = [{ user_id: id, username: 'bob', roles: [h('role')] }];
    await mockAdmin(page, state, {
      'GET /roles': problem(403, 'forbidden'),
      [`GET /users/${encodeURIComponent(id)}/links`]: problem(500, h('links detail')),
      [`GET /users/${encodeURIComponent(id)}/permissions`]: problem(500, h('permissions detail')),
    });
    await page.goto(`/admin/users/${encodeURIComponent(id)}`);
    await expect(page.locator('#admin-user-id')).toContainText(id);
    await expect(page.locator('#admin-user-roles')).toContainText(h('role'));
    await expect(page.locator('#admin-user-links-error')).toContainText(h('links detail'));
    await expect(page.locator('#admin-user-permissions-error')).toContainText(h('permissions detail'));
    await expectNoInjection(page);
  });

  test('the user editor error', async ({ page }) => {
    await mockAdmin(page, adminState(), { [`GET /users/${ID.bob}`]: problem(404, h('detail')) });
    await page.goto(`/admin/users/${ID.bob}`);
    await expect(page.locator('#admin-error')).toContainText(h('detail'));
    await expectNoInjection(page);
  });

  test('the role list', async ({ page }) => {
    const state = adminState();
    state.roles[1] = {
      id: h('id'),
      name: h('name'),
      description: h('description'),
      permissions: [
        { ...PERMS[0], node: h('node') },
        { ...PERMS[1], value: h('number-looking text') },
        { ...PERMS[2], value: [h('item'), 'other'] },
      ],
    };
    await mockAdmin(page, state);
    await page.goto('/admin/roles');
    const row = page.locator('#admin-roles li').nth(1);
    await expect(row).toContainText(h('name'));
    await expect(row).toContainText(h('description'));
    await expect(row).toContainText(h('node'));
    await expect(row).toContainText(h('number-looking text'));
    await expect(row).toContainText(h('item'));
    await expectNoInjection(page);
  });

  test('the role editor', async ({ page }) => {
    const state = adminState();
    state.permissions[4].node = h('ungranted node');
    state.roles[1] = {
      id: ID.bee,
      name: h('name'),
      description: h('description'),
      permissions: [
        { ...PERMS[0], node: h('node'), description: h('node description') },
        { ...PERMS[3], value: h('value') },
        { ...PERMS[2], value: [h('item')] },
      ],
    };
    await mockAdmin(page, state);
    await page.goto(`/admin/roles/${ID.bee}`);
    await expect(page.locator('#admin-role-title')).toContainText(h('name'));
    await expect(page.locator('#admin-role-permissions')).toContainText(h('node'));
    await expect(page.locator('#admin-role-permissions')).toContainText(h('node description'));
    await expect(page.locator('#admin-role-permissions li').nth(1).locator('input')).toHaveValue(h('value'));
    await expect(page.locator('#admin-role-permissions li').nth(2).locator('textarea')).toHaveValue(h('item'));
    await expect(page.locator('#admin-role-grant-permission option').last()).toHaveText(h('ungranted node'));
    await expectNoInjection(page);
  });

  test('the role editor with a hostile ID and a refused action', async ({ page }) => {
    const state = adminState();
    state.roles[1].id = h('role id');
    await mockAdmin(page, state, { [`DELETE /roles/${encodeURIComponent(h('role id'))}/permissions/${ID.pBee}`]: problem(409, h('detail')) });
    await page.goto(`/admin/roles/${encodeURIComponent(h('role id'))}`);
    await expect(page.locator('#admin-role-id')).toContainText(h('role id'));
    await page.locator('#admin-role-permissions li').nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(page.locator('#admin-error')).toContainText(h('detail'));
    await expectNoInjection(page);
  });

  test('the permission list', async ({ page }) => {
    const state = adminState();
    state.permissions[1] = { ...PERMS[1], node: h('node'), description: h('description'), merge: h('merge') };
    state.permissions[2] = { ...PERMS[2], value_type: h('type') };
    await mockAdmin(page, state, { [`DELETE /permissions/${ID.pBee}`]: problem(409, h('detail')) });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/admin/permissions');
    const rows = page.locator('#admin-permissions li');
    await expect(rows.nth(1)).toContainText(h('node'));
    await expect(rows.nth(1)).toContainText(h('description'));
    await expect(rows.nth(1)).toContainText(h('merge'));
    await expect(rows.nth(2)).toContainText(h('type'));
    await rows.nth(0).getByRole('button', { name: 'Delete' }).click();
    await expect(page.locator('#admin-error')).toContainText(h('detail'));
    await expectNoInjection(page);
  });
});
