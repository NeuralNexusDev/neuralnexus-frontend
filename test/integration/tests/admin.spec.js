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
    await expect(page.locator('#admin-denied')).toBeHidden();
  });

  test('an account without admin permissions sees that it has none', async ({ page }) => {
    await mockMyPermissions(page, ['ratelimit:1000']);
    await page.goto('/admin');
    await expect(page.locator('#admin-denied')).toBeVisible();
    await expect(page.locator('#admin-users-link')).toBeHidden();
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
  async function mockList(page, { users = USERS, roles = json(ROLES) } = {}) {
    await page.route(`${API}/users`, (route) => route.fulfill(Array.isArray(users) ? json(users) : users));
    await page.route(`${API}/roles`, (route) => route.fulfill(roles));
  }

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
