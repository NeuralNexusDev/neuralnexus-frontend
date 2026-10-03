import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;

const json = (body, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(body) });
const problem = (status, detail) => ({
  status,
  contentType: 'application/problem+json',
  body: JSON.stringify({ title: 'x', status, detail }),
});

async function mockMyPermissions(page, permissions, status = 200) {
  await page.route(`${API}/users/me/permissions`, (route) =>
    route.fulfill(status === 200 ? json(permissions) : problem(status, 'no access'))
  );
}

async function mockAccountPage(page, permissions) {
  await page.route(`${API}/users/me`, (route) => route.fulfill(json({ username: 'admin' })));
  await page.route(`${API}/users/me/links`, (route) => route.fulfill(json([])));
  await page.route(`${API}/users/me/settings`, (route) => route.fulfill(json({ password_auth: true })));
  await mockMyPermissions(page, permissions);
}

const ROLES = [
  { id: '1', name: 'system', description: 'System', permissions: [] },
  { id: '3', name: 'bee_admin', description: 'Bee Name Generator Admin', permissions: [] },
];

const USERS = [
  { user_id: '100', username: 'alice', roles: ['1'] },
  { user_id: '200', username: 'bob', roles: ['3', '1'] },
  { user_id: '300', username: '', roles: [] },
];

test.describe('admin - settings link', () => {
  for (const permissions of [['users.admin'], ['roles.admin'], ['ratelimit:1000', 'users.admin']]) {
    test(`the account page links to the admin dashboard with ${permissions.join(', ')}`, async ({ page }) => {
      await mockAccountPage(page, permissions);
      await page.goto('/account');
      const link = page.locator('#admin-dashboard-link');
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute('href', '/admin');
    });
  }

  for (const permissions of [[], ['ratelimit:1000'], ['beenamegenerator.admin'], ['users.administrator']]) {
    test(`the account page has no admin link with [${permissions.join(', ')}]`, async ({ page }) => {
      await mockAccountPage(page, permissions);
      await page.goto('/account');
      await expect(page.locator('#account-username')).toHaveText('admin');
      await expect(page.locator('#admin-dashboard-link')).toBeHidden();
    });
  }
});

test.describe('admin - dashboard', () => {
  test('users.admin shows the users card', async ({ page }) => {
    await mockMyPermissions(page, ['users.admin']);
    await page.goto('/admin');
    await expect(page.locator('#admin-users-link')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toHaveAttribute('href', '/admin/users');
    await expect(page.locator('#admin-roles-link')).toBeHidden();
    await expect(page.locator('#admin-permissions-link')).toBeHidden();
    await expect(page.locator('#admin-denied')).toBeHidden();
  });

  test('roles.admin shows the roles and permissions cards', async ({ page }) => {
    await mockMyPermissions(page, ['roles.admin']);
    await page.goto('/admin');
    await expect(page.locator('#admin-roles-link')).toHaveAttribute('href', '/admin/roles');
    await expect(page.locator('#admin-permissions-link')).toHaveAttribute('href', '/admin/permissions');
    await expect(page.locator('#admin-users-link')).toBeHidden();
    await expect(page.locator('#admin-denied')).toBeHidden();
  });

  test('an account without admin permissions sees that it has none', async ({ page }) => {
    await mockMyPermissions(page, ['ratelimit:1000']);
    await page.goto('/admin');
    await expect(page.locator('#admin-denied')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toBeHidden();
    await expect(page.locator('#admin-roles-link')).toBeHidden();
    await expect(page.locator('#admin-permissions-link')).toBeHidden();
  });

  test('a failed permissions lookup shows the API message', async ({ page }) => {
    await mockMyPermissions(page, [], 500);
    await page.goto('/admin');
    await expect(page.locator('#admin-error')).toHaveText('no access');
    await expect(page.locator('#admin-users-link')).toBeHidden();
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await page.route(`${API}/users/me/permissions`, (route) => route.fulfill(problem(401, 'sign in')));
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto('/admin');
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('admin - user list', () => {
  const listUrl = new RegExp(`^${API.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/users\\?`);

  async function mockList(page, { users = USERS, roles = json(ROLES) } = {}) {
    const requests = [];
    await page.route(listUrl, (route) => {
      const url = new URL(route.request().url());
      requests.push(url.search);
      if (!Array.isArray(users)) {
        return route.fulfill(typeof users === 'function' ? users(requests.length) : users);
      }
      const limit = Number(url.searchParams.get('limit'));
      const offset = Number(url.searchParams.get('offset'));
      return route.fulfill(json(users.slice(offset, offset + limit)));
    });
    await page.route(`${API}/roles`, (route) => route.fulfill(roles));
    return requests;
  }

  const manyUsers = (count) => Array.from({ length: count }, (_, i) => ({ user_id: String(i + 1), username: `user${i + 1}`, roles: [] }));

  test('lists every user with a link to its editor and role names', async ({ page }) => {
    await mockList(page);
    await page.goto('/admin/users');
    const rows = page.locator('#admin-users li');
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText('alice');
    await expect(rows.nth(0)).toContainText('100');
    await expect(rows.nth(0).locator('a')).toHaveAttribute('href', '/admin/users/100');
    await expect(rows.nth(1).locator('span.rounded-full')).toHaveText(['bee_admin', 'system']);
    await expect(rows.nth(2)).toContainText('No username');
    await expect(page.locator('#admin-users-empty')).toBeHidden();
  });

  test('the search box filters by username or user ID', async ({ page }) => {
    await mockList(page);
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(3);

    await page.locator('#admin-users-search').fill('BO');
    await expect(page.locator('#admin-users li')).toHaveCount(1);
    await expect(page.locator('#admin-users li')).toContainText('bob');

    await page.locator('#admin-users-search').fill('300');
    await expect(page.locator('#admin-users li')).toHaveCount(1);
    await expect(page.locator('#admin-users li')).toContainText('No username');

    await page.locator('#admin-users-search').fill('nobody');
    await expect(page.locator('#admin-users li')).toHaveCount(0);
    await expect(page.locator('#admin-users-empty')).toBeVisible();
  });

  test('role IDs are shown when the caller cannot list roles', async ({ page }) => {
    await mockList(page, { roles: problem(403, 'forbidden') });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li').nth(1).locator('span.rounded-full')).toHaveText(['3', '1']);
  });

  test('usernames are rendered as text, never as HTML', async ({ page }) => {
    await mockList(page, { users: [{ user_id: '1', username: '<img src=x onerror="window.__xss=1">', roles: [] }] });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toContainText('<img src=x onerror="window.__xss=1">');
    await expect(page.locator('#admin-users img')).toHaveCount(0);
    expect(await page.evaluate(() => window.__xss)).toBeUndefined();
  });

  test('loads a page at a time and offers more only while a page comes back full', async ({ page }) => {
    const requests = await mockList(page, { users: manyUsers(250) });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    await expect(page.locator('#admin-users-more')).toBeVisible();

    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users li')).toHaveCount(250);
    await expect(page.locator('#admin-users-more')).toBeHidden();
    expect(requests).toEqual(['?limit=200&offset=0', '?limit=200&offset=200']);
    await expect(page.locator('#admin-users li').last()).toContainText('user250');
  });

  test('a list of exactly one full page still offers more, and an empty next page ends it', async ({ page }) => {
    const requests = await mockList(page, { users: manyUsers(200) });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    await expect(page.locator('#admin-users-more')).toBeVisible();
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users-more')).toBeHidden();
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    expect(requests).toHaveLength(2);
  });

  test('a short first page offers no more', async ({ page }) => {
    await mockList(page);
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(3);
    await expect(page.locator('#admin-users-more')).toBeHidden();
  });

  test('search covers the users loaded after a Load more', async ({ page }) => {
    await mockList(page, { users: manyUsers(250) });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    await page.locator('#admin-users-search').fill('user250');
    await expect(page.locator('#admin-users li')).toHaveCount(0);
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users li')).toHaveCount(1);
  });

  test('a failed Load more shows the API message, keeps the list and can be retried', async ({ page }) => {
    await mockList(page, { users: (call) => (call === 1 ? json(manyUsers(200)) : problem(500, 'Failed to list users')) });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-error')).toHaveText('Failed to list users');
    await expect(page.locator('#admin-users li')).toHaveCount(200);
    await expect(page.locator('#admin-users-more-button')).toBeEnabled();
  });

  test('a refused list shows the API message', async ({ page }) => {
    await mockList(page, { users: problem(403, 'You do not have permission to list users') });
    await page.goto('/admin/users');
    await expect(page.locator('#admin-error')).toHaveText('You do not have permission to list users');
    await expect(page.locator('#admin-users li')).toHaveCount(0);
  });
});

test.describe('admin - user editor', () => {
  const USER = { user_id: '200', username: 'bob', roles: ['3'] };
  const LINKS = [
    { platform: 'discord', platform_username: 'bob#1234', platform_id: '9', verified: true, login_enabled: true },
    { platform: 'steam', platform_id: '76561198000000000', verified: true, login_enabled: false },
  ];

  async function mockUser(page, { roles = json(ROLES), put } = {}) {
    const puts = [];
    let permissions = ['beenamegenerator.admin'];
    await page.route(`${API}/users/200`, (route) => {
      if (route.request().method() === 'PUT') {
        puts.push(route.request().postDataJSON());
        const response = put ? put(puts.at(-1)) : json({ ...USER, ...puts.at(-1) });
        if (response.status < 300) {
          permissions = ['beenamegenerator.admin', 'users.admin'];
        }
        return route.fulfill(response);
      }
      return route.fulfill(json(USER));
    });
    await page.route(`${API}/users/200/links`, (route) => route.fulfill(json(LINKS)));
    await page.route(`${API}/users/200/permissions`, (route) => route.fulfill(json(permissions)));
    await page.route(`${API}/roles`, (route) => route.fulfill(roles));
    return puts;
  }

  test('shows the username, roles, linked accounts and effective permissions', async ({ page }) => {
    await mockUser(page);
    await page.goto('/admin/users/200');
    await expect(page.locator('#admin-user-title')).toHaveText('bob');
    await expect(page.locator('#admin-user-id')).toHaveText('200');
    await expect(page.locator('#admin-user-username')).toHaveValue('bob');
    const boxes = page.locator('#admin-user-roles input');
    await expect(boxes).toHaveCount(2);
    await expect(boxes.nth(0)).not.toBeChecked();
    await expect(boxes.nth(1)).toBeChecked();
    await expect(page.locator('#admin-user-roles li').nth(1)).toContainText('Bee Name Generator Admin');
    await expect(page.locator('#admin-user-links li')).toHaveText(['discordbob#1234', 'steam76561198000000000']);
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin']);
    await expect(page.locator('#admin-user-roles-note')).toBeHidden();
  });

  test('saving sends the trimmed username and the ticked roles, then refreshes the permissions', async ({ page }) => {
    const puts = await mockUser(page);
    await page.goto('/admin/users/200');
    await expect(page.locator('#admin-user-form')).toBeVisible();
    await page.locator('#admin-user-username').fill('  robert  ');
    await page.locator('#admin-user-roles input').nth(0).check();
    await page.locator('#admin-user-save').click();

    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(puts).toEqual([{ username: 'robert', roles: ['1', '3'] }]);
    await expect(page.locator('#admin-user-title')).toHaveText('robert');
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin', 'users.admin']);
    await expect(page.locator('#admin-user-save')).toBeEnabled();
  });

  test('unticking every role sends an empty list', async ({ page }) => {
    const puts = await mockUser(page);
    await page.goto('/admin/users/200');
    await page.locator('#admin-user-roles input').nth(1).uncheck();
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(puts).toEqual([{ username: 'bob', roles: [] }]);
  });

  test('a refused save shows the API message and keeps the form editable', async ({ page }) => {
    await mockUser(page, { put: () => problem(409, 'An account with this username already exists') });
    await page.goto('/admin/users/200');
    await page.locator('#admin-user-username').fill('alice');
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-error')).toHaveText('An account with this username already exists');
    await expect(page.locator('#admin-user-status')).toBeHidden();
    await expect(page.locator('#admin-user-save')).toBeEnabled();
  });

  test('without roles.admin the roles are listed by ID and left out of the save', async ({ page }) => {
    const puts = await mockUser(page, { roles: problem(403, 'forbidden') });
    await page.goto('/admin/users/200');
    await expect(page.locator('#admin-user-roles li')).toHaveText(['3']);
    await expect(page.locator('#admin-user-roles input')).toHaveCount(0);
    await expect(page.locator('#admin-user-roles-note')).toBeVisible();
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(puts).toEqual([{ username: 'bob' }]);
  });

  test('an unknown user shows the API message and no form', async ({ page }) => {
    await page.route(`${API}/users/999**`, (route) => route.fulfill(problem(404, 'User not found')));
    await page.route(`${API}/roles`, (route) => route.fulfill(json(ROLES)));
    await page.goto('/admin/users/999');
    await expect(page.locator('#admin-error')).toHaveText('User not found');
    await expect(page.locator('#admin-user-form')).toBeHidden();
  });

  test('an encoded user ID in the path is decoded for the requests', async ({ page }) => {
    const requested = [];
    await page.route(`${API}/users/**`, (route) => {
      requested.push(new URL(route.request().url()).pathname);
      return route.fulfill(problem(404, 'User not found'));
    });
    await page.route(`${API}/roles`, (route) => route.fulfill(json(ROLES)));
    await page.goto('/admin/users/a%2Fb');
    await expect(page.locator('#admin-user-id')).toHaveText('a/b');
    expect(requested).toContain(`${new URL(API).pathname}/users/a%2Fb`);
  });
});

const PERMS = [
  { id: '1', node: 'beenamegenerator.admin', description: 'Bee name generator' },
  { id: '3', node: 'ratelimit', description: 'Rate limit', value_type: 'int', merge: 'max' },
  { id: '8', node: 'petpictures.pets', description: 'Pet pictures', value_type: 'string_list', merge: 'union' },
  { id: '9', node: 'motd', description: 'Message of the day', value_type: 'string', merge: 'first' },
  { id: '10', node: 'datastore.admin', description: 'Data store' },
];

function rbacState() {
  return {
    permissions: structuredClone(PERMS),
    roles: [
      { id: '1', name: 'system', description: 'System', permissions: [] },
      { id: '3', name: 'bee_admin', description: 'Bee Name Generator Admin', permissions: [{ ...PERMS[0] }, { ...PERMS[1], value: 100 }] },
    ],
    calls: [],
  };
}

async function mockRbac(page, state, overrides = {}) {
  await page.route(/\/api\/v1\/(roles|permissions)(\/.*)?$/, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.slice(new URL(API).pathname.length);
    const method = request.method();
    const body = request.postData() ? request.postDataJSON() : null;
    state.calls.push({ method, path, body });
    const override = overrides[`${method} ${path}`];
    if (override) {
      return route.fulfill(override);
    }
    const parts = path.split('/').filter(Boolean);
    const role = parts[0] === 'roles' ? state.roles.find((candidate) => candidate.id === parts[1]) : undefined;
    if (method === 'GET' && path === '/roles') {
      return route.fulfill(json(state.roles));
    }
    if (method === 'GET' && path === '/permissions') {
      return route.fulfill(json(state.permissions));
    }
    if (method === 'POST' && path === '/roles') {
      const created = { id: '9', name: body.name, description: body.description || '', permissions: [] };
      state.roles.push(created);
      return route.fulfill(json(created, 201));
    }
    if (method === 'POST' && path === '/permissions') {
      const created = { id: '20', ...body, description: body.description || '' };
      state.permissions.push(created);
      return route.fulfill(json(created, 201));
    }
    if (method === 'DELETE' && parts[0] === 'permissions') {
      state.permissions = state.permissions.filter((candidate) => candidate.id !== parts[1]);
      return route.fulfill({ status: 204 });
    }
    if (!role) {
      return route.fulfill(problem(404, 'Role not found'));
    }
    if (parts.length === 2 && method === 'GET') {
      return route.fulfill(json(role));
    }
    if (parts.length === 2 && method === 'PATCH') {
      Object.assign(role, body);
      return route.fulfill(json(role));
    }
    if (parts.length === 2 && method === 'DELETE') {
      state.roles = state.roles.filter((candidate) => candidate !== role);
      return route.fulfill({ status: 204 });
    }
    const permission = state.permissions.find((candidate) => candidate.id === parts[3]);
    if (method === 'PUT') {
      role.permissions = role.permissions.filter((candidate) => candidate.id !== permission.id);
      role.permissions.push(body ? { ...permission, value: body.value } : { ...permission });
      return route.fulfill({ status: 204 });
    }
    role.permissions = role.permissions.filter((candidate) => candidate.id !== permission.id);
    return route.fulfill({ status: 204 });
  });
}

const writes = (state) => state.calls.filter((call) => call.method !== 'GET');

test.describe('admin - roles list', () => {
  test('lists the roles with their permissions and a link to each editor', async ({ page }) => {
    await mockRbac(page, rbacState());
    await page.goto('/admin/roles');
    const rows = page.locator('#admin-roles li');
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(1)).toContainText('bee_admin');
    await expect(rows.nth(1)).toContainText('Bee Name Generator Admin');
    await expect(rows.nth(1).locator('span.rounded-full')).toHaveText(['beenamegenerator.admin', 'ratelimit: 100']);
    await expect(rows.nth(1).locator('a')).toHaveAttribute('href', '/admin/roles/3');
    await expect(page.locator('#admin-roles-empty')).toBeHidden();
  });

  test('a refused list shows the API message', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'GET /roles': problem(403, 'You do not have permission to manage roles and permissions') });
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-error')).toHaveText('You do not have permission to manage roles and permissions');
  });

  test('creating a role posts it and opens its editor', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill(' moderator ');
    await page.locator('#admin-role-create-description').fill('Moderates things');
    await page.locator('#admin-role-create-submit').click();
    await expect(page).toHaveURL(/\/admin\/roles\/9$/);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/roles', body: { name: 'moderator', description: 'Moderates things' } }]);
    await expect(page.locator('#admin-role-name')).toHaveValue('moderator');
  });

  test('a refused create shows the API message and stays on the list', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'POST /roles': problem(409, 'A role with that name already exists') });
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill('system');
    await page.locator('#admin-role-create-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('A role with that name already exists');
    await expect(page).toHaveURL(/\/admin\/roles$/);
    await expect(page.locator('#admin-role-create-submit')).toBeEnabled();
  });
});

test.describe('admin - role editor', () => {
  const granted = (page) => page.locator('#admin-role-permissions li');

  test('shows the role, its granted permissions with values, and the ones still to grant', async ({ page }) => {
    await mockRbac(page, rbacState());
    await page.goto('/admin/roles/3');
    await expect(page.locator('#admin-role-title')).toHaveText('bee_admin');
    await expect(page.locator('#admin-role-id')).toHaveText('3');
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_admin');
    await expect(page.locator('#admin-role-description')).toHaveValue('Bee Name Generator Admin');
    await expect(granted(page)).toHaveCount(2);
    await expect(granted(page).nth(0)).toContainText('beenamegenerator.admin');
    await expect(granted(page).nth(0).locator('input')).toHaveCount(0);
    await expect(granted(page).nth(1).locator('input')).toHaveValue('100');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd', 'datastore.admin']);
  });

  test('saving sends the trimmed name and description and shows Saved', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await expect(page.locator('#admin-role-content')).toBeVisible();
    await page.locator('#admin-role-name').fill(' bee_manager ');
    await page.locator('#admin-role-description').fill(' Manages bees ');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    expect(writes(state)).toEqual([{ method: 'PATCH', path: '/roles/3', body: { name: 'bee_manager', description: 'Manages bees' } }]);
    await expect(page.locator('#admin-role-title')).toHaveText('bee_manager');
  });

  test('a refused save shows the API message', async ({ page }) => {
    await mockRbac(page, rbacState(), {
      'PATCH /roles/1': problem(409, 'Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin'),
    });
    await page.goto('/admin/roles/1');
    await page.locator('#admin-role-name').fill('renamed');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-error')).toHaveText('Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin');
    await expect(page.locator('#admin-role-status')).toBeHidden();
    await expect(page.locator('#admin-role-save')).toBeEnabled();
  });

  test('changing an int value puts the whole number', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await granted(page).nth(1).locator('input').fill('250');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('250');
    expect(writes(state)).toEqual([{ method: 'PUT', path: '/roles/3/permissions/3', body: { value: 250 } }]);
  });

  for (const bad of ['', '1.5']) {
    test(`an int value of "${bad}" is refused without a request`, async ({ page }) => {
      const state = rbacState();
      await mockRbac(page, state);
      await page.goto('/admin/roles/3');
      await granted(page).nth(1).locator('input').fill(bad);
      await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
      await expect(page.locator('#admin-error')).toHaveText('Enter a whole number');
      expect(writes(state)).toEqual([]);
    });
  }

  test('granting a permission without a value puts it bare', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await expect(page.locator('#admin-role-grant-value input, #admin-role-grant-value textarea')).toHaveCount(0);
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: '/roles/3/permissions/10', body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd']);
  });

  test('granting a list permission puts the trimmed non-empty lines', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'petpictures.pets' });
    await page.locator('#admin-role-grant-value textarea').fill('rex\n  fido  \n\nspot');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: '/roles/3/permissions/8', body: { value: ['rex', 'fido', 'spot'] } }]);
    await expect(granted(page).nth(2).locator('textarea')).toHaveValue('rex\nfido\nspot');
  });

  test('granting a text permission puts the trimmed text', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('  hello  ');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(writes(state)).toEqual([{ method: 'PUT', path: '/roles/3/permissions/9', body: { value: 'hello' } }]);
  });

  test('a list or text value left empty is refused without a request', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'petpictures.pets' });
    await page.locator('#admin-role-grant-value textarea').fill('  \n ');
    await page.locator('#admin-role-grant-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('Enter at least one item');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('Enter a value');
    expect(writes(state)).toEqual([]);
  });

  test('removing a permission deletes the grant', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    expect(writes(state)).toEqual([{ method: 'DELETE', path: '/roles/3/permissions/1', body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toContainText(['beenamegenerator.admin']);
  });

  test('a refused removal shows the API message and keeps the grant', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'DELETE /roles/3/permissions/1': problem(409, 'system and owner keep roles.admin') });
    await page.goto('/admin/roles/3');
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(page.locator('#admin-error')).toHaveText('system and owner keep roles.admin');
    await expect(granted(page)).toHaveCount(2);
  });

  test('a role holding every permission offers nothing more to grant', async ({ page }) => {
    const state = rbacState();
    state.roles[1].permissions = state.permissions.map((permission) => ({ ...permission }));
    await mockRbac(page, state);
    await page.goto('/admin/roles/3');
    await expect(page.locator('#admin-role-grant-form')).toBeHidden();
    await expect(page.locator('#admin-role-grant-empty')).toBeVisible();
  });

  test('deleting a role asks first, then deletes it and returns to the list', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-delete').click();
    await expect.poll(() => messages.length).toBe(1);
    expect(writes(state)).toEqual([]);

    await page.locator('#admin-role-delete').click();
    await expect(page).toHaveURL(/\/admin\/roles$/);
    expect(messages).toEqual(['Delete the role bee_admin?', 'Delete the role bee_admin?']);
    expect(writes(state)).toEqual([{ method: 'DELETE', path: '/roles/3', body: null }]);
  });

  test('a refused role delete shows the API message', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'DELETE /roles/3': problem(409, 'The role is assigned to an account') });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/admin/roles/3');
    await page.locator('#admin-role-delete').click();
    await expect(page.locator('#admin-error')).toHaveText('The role is assigned to an account');
    await expect(page).toHaveURL(/\/admin\/roles\/3$/);
    await expect(page.locator('#admin-role-delete')).toBeEnabled();
  });

  test('an unknown role shows the API message and no editor', async ({ page }) => {
    await mockRbac(page, rbacState());
    await page.goto('/admin/roles/404');
    await expect(page.locator('#admin-error')).toHaveText('Role not found');
    await expect(page.locator('#admin-role-content')).toBeHidden();
  });
});

test.describe('admin - permissions', () => {
  const rows = (page) => page.locator('#admin-permissions li');

  test('lists the permissions with their value type and merge rule', async ({ page }) => {
    await mockRbac(page, rbacState());
    await page.goto('/admin/permissions');
    await expect(rows(page)).toHaveCount(5);
    await expect(rows(page).nth(0)).toContainText('beenamegenerator.admin');
    await expect(rows(page).nth(0).locator('span.rounded-full')).toHaveCount(0);
    await expect(rows(page).nth(1).locator('span.rounded-full')).toHaveText('int, merge max');
    await expect(rows(page).nth(2).locator('span.rounded-full')).toHaveText('string_list, merge union');
  });

  test('creating a permission without a value sends only the node and description', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill(' pets.write ');
    await page.locator('#admin-permission-create-description').fill('Write pets');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'pets.write', description: 'Write pets' } }]);
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('');
  });

  test('an int permission asks for the merge rule and sends it', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
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
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-type').selectOption('string');
    await page.locator('#admin-permission-create-node').fill('greeting');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'greeting', description: '', value_type: 'string' } }]);
  });

  test('a switch back to no value drops the merge rule', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-type').selectOption('int');
    await page.locator('#admin-permission-create-type').selectOption('');
    await page.locator('#admin-permission-create-node').fill('plain');
    await page.locator('#admin-permission-create-submit').click();
    await expect(rows(page)).toHaveCount(6);
    expect(writes(state)[0].body).toEqual({ node: 'plain', description: '' });
  });

  test('a refused create shows the API message and keeps the form', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'POST /permissions': problem(400, 'Nodes are lower-case words') });
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill('Bad Node');
    await page.locator('#admin-permission-create-submit').click();
    await expect(page.locator('#admin-error')).toHaveText('Nodes are lower-case words');
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('Bad Node');
    await expect(page.locator('#admin-permission-create-submit')).toBeEnabled();
  });

  test('deleting a permission asks first, then deletes it', async ({ page }) => {
    const state = rbacState();
    await mockRbac(page, state);
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
    expect(writes(state)).toEqual([{ method: 'DELETE', path: '/permissions/10', body: null }]);
  });

  test('a refused delete shows the API message and keeps the permission', async ({ page }) => {
    await mockRbac(page, rbacState(), { 'DELETE /permissions/1': problem(409, 'The permission is granted by a role') });
    page.on('dialog', (dialog) => dialog.accept());
    await page.goto('/admin/permissions');
    await rows(page).nth(0).getByRole('button', { name: 'Delete' }).click();
    await expect(page.locator('#admin-error')).toHaveText('The permission is granted by a role');
    await expect(rows(page)).toHaveCount(5);
  });
});
