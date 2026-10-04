import { test, expect, signIn, ID, HOSTILE, error, expectNoInjection } from './admin-helpers.js';

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

test.describe('admin - user changes', () => {
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

  test('Load more moves focus to the first new user', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users > li:has(> a)')).toHaveCount(250);
    await expect(page.locator('#admin-users > li:has(> a)').nth(200).locator('a')).toBeFocused();
  });
});
