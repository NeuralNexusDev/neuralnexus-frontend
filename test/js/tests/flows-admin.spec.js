import { test, expect, signIn, ID, error } from './helpers.js';

const HOSTILE = '"><img src=x onerror="window.__xss=1">';

async function expectNoInjection(page) {
  await expect(page.locator('img[src="x"]')).toHaveCount(0);
  expect(await page.evaluate(() => window.__xss)).toBeUndefined();
}

const userRows = (page) => page.locator('#admin-users > li:has(> a)');
const search = (page) => page.locator('#admin-users-search');
const userEditor = `/admin/users/${ID.bob}`;
const username = (page) => page.locator('#admin-user-username');
const userSave = (page) => page.locator('#admin-user-save');
const userStatus = (page) => page.locator('#admin-user-status');

const roleEditor = `/admin/roles/${ID.bee}`;
const granted = (page) => page.locator('#admin-role-permissions li');
const heading = (page) => page.locator('#admin-role-permissions-title');
const rateField = (page) => granted(page).nth(1).locator('input');

const permissionRows = (page) => page.locator('#admin-permissions li');

test.describe('users list', () => {
  test('FA-01 the list shows each user with a link to its editor and its role names', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    await expect(userRows(page).nth(0)).toContainText('alice');
    await expect(userRows(page).nth(0)).toContainText(ID.alice);
    await expect(userRows(page).nth(0).locator('a')).toHaveAttribute('href', `/admin/users/${ID.alice}`);
    await expect(userRows(page).nth(1).locator('span.rounded-full')).toHaveText(['bee_admin']);
    await expect(userRows(page).nth(2)).toContainText('No username');
  });

  test('FA-02 the search box asks the server, matching username or user ID, ignoring case and surrounding spaces', async ({ page }) => {
    const { calls } = await signIn(page);
    await page.goto('/admin/users');
    await search(page).fill('  BO ');
    await expect(userRows(page)).toHaveCount(1);
    await expect(userRows(page)).toContainText('bob');

    await search(page).fill('675903');
    await expect(userRows(page)).toContainText('No username');

    await search(page).fill('nobody');
    await expect(userRows(page)).toHaveCount(0);
    await expect(page.locator('#admin-users-empty')).toHaveText('No users found');

    await search(page).fill('');
    await expect(userRows(page)).toHaveCount(3);
    await expect(page.locator('#admin-users-empty')).toHaveCount(0);
    await expect(search(page)).toHaveValue('');
    expect((await calls()).filter((call) => call.path === '/users').length).toBeGreaterThan(1);
  });

  test('FA-03 a search covers users beyond the first page, and stops asking once the list ends', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(200);
    await search(page).fill('user247');
    await expect(userRows(page)).toHaveCount(1);
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
  });

  test('FA-04 a search that reaches its limit offers to keep looking, and keeps the search text', async ({ page }) => {
    await signIn(page, { generateUsers: 1100 });
    await page.goto('/admin/users');
    await search(page).fill('user1099');
    await expect(page.locator('#admin-users-empty')).toHaveText('No matches in the first 1000 users');
    await page.locator('#admin-users-more-button').click();
    await expect(userRows(page)).toHaveCount(1);
    await expect(userRows(page)).toContainText('user1099');
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
    await expect(search(page)).toHaveValue('user1099');
  });

  test('FA-05 Load more with nothing left to find removes the button and moves focus to a notice', async ({ page }) => {
    await signIn(page, { generateUsers: 1100 });
    await page.goto('/admin/users');
    await search(page).fill('zzz');
    await expect(page.locator('#admin-users-empty')).toHaveText('No matches in the first 1000 users');
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users-end')).toBeFocused();
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
    await expect(userRows(page)).toHaveCount(0);
  });

  test('FA-06 Load more that finds no more users removes the button and moves focus to a notice', async ({ page }) => {
    await signIn(page, { generateUsers: 197 });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users-end')).toBeFocused();
    await expect(page.locator('#admin-users-more-button')).toHaveCount(0);
  });

  test('FA-07 a failed Load more shows the API message, keeps the list, and a retry clears the message', async ({ page }) => {
    await signIn(page, { generateUsers: 247, failures: { 'GET /users': { status: 500, detail: 'Failed to list users', skip: 1, times: 1 } } });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(error(page)).toHaveText('Failed to list users');
    await expect(userRows(page)).toHaveCount(200);

    await page.locator('#admin-users-more-button').click();
    await expect(userRows(page)).toHaveCount(250);
    await expect(error(page)).toHaveText('');
  });

  test('FA-08 Load more moves focus to the first new user', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(200);
    await page.locator('#admin-users-more-button').click();
    await expect(page.locator('#admin-users > li:has(> a)')).toHaveCount(250);
    await expect(page.locator('#admin-users > li:has(> a)').nth(200).locator('a')).toBeFocused();
  });
});

test.describe('user editor', () => {
  test('FA-10 the editor shows the username, roles, linked accounts and effective permissions', async ({ page }) => {
    await signIn(page);
    await page.goto(userEditor);
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

  test('FA-11 saving a new username sends only the username, says Saved in the status line and keeps focus on Save', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(userEditor);
    await username(page).fill('  robert  ');
    await userSave(page).click();
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(page.locator('#admin-user-title')).toHaveText('robert');
    await expect(username(page)).toHaveValue('robert');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
    await expect(userSave(page)).toBeFocused();
    await expect(userStatus(page)).toHaveAttribute('role', 'status');
  });

  test('FA-12 saving with Enter in the username field leaves the saved name in the field', async ({ page }) => {
    await signIn(page);
    await page.goto(userEditor);
    await username(page).fill('robert');
    await username(page).press('Enter');
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(username(page)).toHaveValue('robert');
    await expect(username(page)).toBeFocused();
  });

  test('FA-13 saving without a change sends nothing', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(userEditor);
    await userSave(page).click();
    await expect(userStatus(page)).toHaveText('Nothing to save');
    expect(await writes()).toEqual([]);
  });

  test('FA-14 a refused save shows the API message, keeps what was typed, and a good retry clears the message', async ({ page }) => {
    await signIn(page);
    await page.goto(userEditor);
    await username(page).fill('alice');
    await userSave(page).click();
    await expect(error(page)).toHaveText('An account with this username already exists');
    await expect(username(page)).toHaveValue('alice');
    await expect(userStatus(page)).toHaveText('');

    await username(page).fill('carol');
    await userSave(page).click();
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(error(page)).toHaveText('');
  });

  test('FA-15 a save whose page could not be refreshed still compares the next save with what was stored', async ({ page }) => {
    const { writes } = await signIn(page, { failures: { 'GET /roles': { status: 500, detail: 'roles are down', skip: 1, times: 1 } } });
    await page.goto(`/admin/users/${ID.bob}`);
    await page.locator('#admin-user-username').fill('robert');
    await page.locator('#admin-user-save').click();
    await expect(error(page)).toHaveText('The change was made, but the page could not be refreshed: roles are down');
    await expect(page.locator('#admin-user-title')).toHaveText('robert');

    await page.locator('#admin-user-username').fill('bob');
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    expect(await writes()).toEqual([
      { method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } },
      { method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'bob' } },
    ]);
  });

  test('FA-16 a failed save empties the status line of the save before it', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/users/${ID.bob}`);
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Nothing to save');
    await page.locator('#admin-user-username').fill('alice');
    await page.locator('#admin-user-save').click();
    await expect(error(page)).toHaveText('An account with this username already exists');
    await expect(page.locator('#admin-user-status')).toHaveText('');
  });

  test('FA-17 saving the roles sends them and redraws the effective permissions', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(userEditor);
    await expect(page.locator('#admin-user-permissions li')).toHaveText(['beenamegenerator.admin', 'ratelimit:100']);
    await page.locator(`#admin-user-roles input[value="${ID.bee}"]`).uncheck();
    await userSave(page).click();
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(page.locator('#admin-user-permissions-empty')).toHaveText('No permissions');
    await expect(page.locator('#admin-user-permissions li')).toHaveCount(0);
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [] } }]);
  });
});

test.describe('roles list', () => {
  test('FA-20 the list shows each role with its permissions and a link to its editor', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    const rows = page.locator('#admin-roles li');
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(2)).toContainText('bee_admin');
    await expect(rows.nth(2).locator('span.rounded-full')).toHaveText(['beenamegenerator.admin', 'ratelimit: 100']);
    await expect(rows.nth(2).locator('a')).toHaveAttribute('href', roleEditor);
  });

  test('FA-21 creating a role opens its editor', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto('/admin/roles');
    await page.locator('#admin-role-create-name').fill(' moderator ');
    await page.locator('#admin-role-create-description').fill('Moderates things');
    await page.locator('#admin-role-create-submit').click();
    await expect(page).toHaveURL(/\/admin\/roles\/\d{19}$/);
    await expect(page.locator('#admin-role-name')).toHaveValue('moderator');
    expect(await writes()).toEqual([{ method: 'POST', path: '/roles', body: { name: 'moderator', description: 'Moderates things' } }]);
  });
});

test.describe('role editor', () => {
  test('FA-30 the editor shows the role, its grants with values, and the permissions still to grant', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-title')).toHaveText('bee_admin');
    await expect(page.locator('#admin-role-id')).toHaveText(ID.bee);
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_admin');
    await expect(granted(page)).toHaveCount(2);
    await expect(granted(page).nth(1).locator('input[type="number"]')).toHaveValue('100');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd', 'datastore.admin']);
  });

  test('FA-31 changing an int value puts the whole number and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await granted(page).nth(1).locator('input').fill('250');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('250');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
    await expect(heading(page)).toBeFocused();
  });

  test('FA-32 granting a permission without a value puts it bare', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await expect(page.locator('#admin-role-grant-value').locator('input, textarea')).toHaveCount(0);
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd']);
  });

  test('FA-33 choosing a permission swaps in the input for its type, and granting puts the typed value', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
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

  test('FA-34 granting an int value puts the whole number once its permission is free', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await granted(page).nth(1).getByRole('button', { name: 'Remove ratelimit' }).click();
    await expect(granted(page)).toHaveCount(1);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'ratelimit' });
    await page.locator('#admin-role-grant-value input').fill('-7');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(2);
    expect((await writes()).at(-1)).toEqual({ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: -7 } });
  });

  test('FA-35 removing a permission deletes the grant by its exact ID and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(roleEditor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toContainText(['beenamegenerator.admin']);
    await expect(heading(page)).toBeFocused();
  });

  test('FA-36 a removal whose editor could not be loaded again offers the permission to grant again', async ({ page }) => {
    await signIn(page, { failures: { [`GET /roles/${ID.bee}`]: { status: 500, detail: 'roles are down', skip: 1, times: 1 } } });
    await page.goto(roleEditor);
    await page.getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(error(page)).toHaveText('The change was made, but the page could not be refreshed: roles are down');
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['beenamegenerator.admin', 'petpictures.pets', 'motd', 'datastore.admin']);
  });

  test('FA-37 a grant whose editor could not be loaded again lists the permission and stops offering it', async ({ page }) => {
    const { writes } = await signIn(page, { failures: { [`GET /roles/${ID.bee}`]: { status: 500, detail: 'roles are down', skip: 1, times: 1 } } });
    await page.goto(roleEditor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(error(page)).toHaveText('The change was made, but the page could not be refreshed: roles are down');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pStore}`, body: null }]);
    await expect(granted(page)).toHaveCount(3);
    await expect(granted(page).nth(2)).toContainText('datastore.admin');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'motd']);
  });

  test('FA-38 a half-typed grant value survives removing another permission', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('half typed');
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission')).toHaveValue(ID.pMotd);
    await expect(page.locator('#admin-role-grant-value input')).toHaveValue('half typed');
  });

  test('FA-39 a refused removal shows the API message, and the next action clears it', async ({ page }) => {
    await signIn(page, {
      failures: { [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: { status: 409, detail: 'system and owner keep roles.admin', times: 1 } },
    });
    await page.goto(roleEditor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(error(page)).toHaveText('system and owner keep roles.admin');
    await expect(granted(page)).toHaveCount(2);

    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(error(page)).toHaveText('');
  });

  test('FA-40 renaming a role keeps an unsaved value and updates the title and the delete prompt', async ({ page }) => {
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

  test('FA-41 granting a permission keeps an unsaved value in another row and empties the grant form', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await rateField(page).fill('555');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input').fill('hello');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    await expect(rateField(page)).toHaveValue('555');
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['petpictures.pets', 'datastore.admin']);
    await expect(page.locator('#admin-role-grant-value textarea')).toHaveValue('');
  });

  test('FA-42 saving one value keeps the unsaved value in another row', async ({ page }) => {
    const { writes } = await signIn(page, {
      roles: [{ id: ID.bee, name: 'bee_admin', description: 'd', grants: [{ id: ID.pRate, value: 100 }, { id: ID.pMotd, value: 'hi' }] }],
    });
    await page.goto(roleEditor);
    await granted(page).nth(0).locator('input').fill('250');
    await granted(page).nth(1).locator('input').fill('half typed');
    await granted(page).nth(0).getByRole('button', { name: 'Save value of ratelimit' }).click();
    await expect.poll(writes).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
    await expect(granted(page).nth(0).locator('input')).toHaveValue('250');
    await expect(granted(page).nth(1).locator('input')).toHaveValue('half typed');
  });

  test('FA-43 the last grant moves focus to the list heading', async ({ page }) => {
    await signIn(page, {
      roles: [{ id: ID.bee, name: 'bee_admin', description: 'd', grants: [{ id: ID.pRate, value: 1 }, { id: ID.pPets, value: ['a'] }, { id: ID.pMotd, value: 'x' }, { id: ID.pStore }] }],
    });
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['beenamegenerator.admin']);
    await page.locator('#admin-role-grant-submit').click();
    await expect(page.locator('#admin-role-grant-empty')).toBeVisible();
    await expect(page.locator('#admin-role-permissions-title')).toBeFocused();
  });

  test('FA-43 a grant that leaves permissions to grant keeps focus on the Grant button', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'datastore.admin' });
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    await expect(page.locator('#admin-role-grant-submit')).toBeFocused();
  });

  test('FA-44 a role change empties the status line of the rename before it', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    await granted(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-status')).toHaveText('');
  });

  test('FA-45 deleting a role asks first, then deletes it and returns to the list', async ({ page }) => {
    const { writes } = await signIn(page, { users: [] });
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto(roleEditor);
    await page.locator('#admin-role-delete').click();
    await expect.poll(() => messages.length).toBe(1);
    expect(await writes()).toEqual([]);

    await page.locator('#admin-role-delete').click();
    await expect(page).toHaveURL(/\/admin\/roles$/);
    expect(messages).toEqual(['Delete the role bee_admin?', 'Delete the role bee_admin?']);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}`, body: null }]);
  });

  test('FA-46 API text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, {
      permissions: [
        { id: ID.pBee, node: HOSTILE, description: HOSTILE },
        { id: ID.pMotd, node: 'motd', description: HOSTILE, value_type: 'string', merge: 'first' },
        { id: ID.pStore, node: HOSTILE + 'x', description: 'd' },
      ],
      roles: [{ id: ID.bee, name: HOSTILE, description: HOSTILE, grants: [{ id: ID.pBee }, { id: ID.pMotd, value: HOSTILE }] }],
    });
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-title')).toContainText(HOSTILE);
    await expect(granted(page).nth(0)).toContainText(HOSTILE);
    await expect(granted(page).nth(1).locator('input')).toHaveValue(HOSTILE);
    await expectNoInjection(page);
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-roles li').first()).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});

test.describe('permissions', () => {
  test('FA-50 the list shows the permissions with their value type and merge rule', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await expect(permissionRows(page)).toHaveCount(5);
    await expect(permissionRows(page).nth(0).locator('span.rounded-full')).toHaveCount(0);
    await expect(permissionRows(page).nth(1).locator('span.rounded-full')).toHaveText('int, merge max');
    await expect(permissionRows(page).nth(2).locator('span.rounded-full')).toHaveText('string_list, merge union');
  });

  test('FA-51 creating a permission without a value adds it and resets the form', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill(' pets.write ');
    await page.locator('#admin-permission-create-description').fill('Write pets');
    await page.locator('#admin-permission-create-submit').click();
    await expect(permissionRows(page)).toHaveCount(6);
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('');
    expect(await writes()).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'pets.write', description: 'Write pets' } }]);
  });

  test('FA-52 a create whose list could not be loaded again still lists the new permission', async ({ page }) => {
    await signIn(page, { failures: { 'GET /permissions': { status: 500, detail: 'permissions are down', skip: 1, times: 1 } } });
    await page.goto('/admin/permissions');
    await expect(permissionRows(page)).toHaveCount(5);
    await page.locator('#admin-permission-create-node').fill('pets.write');
    await page.locator('#admin-permission-create-submit').click();
    await expect(error(page)).toHaveText('The change was made, but the page could not be refreshed: permissions are down');
    await expect(permissionRows(page)).toHaveCount(6);
    await expect(permissionRows(page).last()).toContainText('pets.write');
  });

  test('FA-53 the merge rule only appears for a whole-number permission, and is sent with it', async ({ page }) => {
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
    await expect(permissionRows(page)).toHaveCount(6);
    expect(await writes()).toEqual([{ method: 'POST', path: '/permissions', body: { node: 'quota', description: '', value_type: 'int', merge: 'min' } }]);
  });

  test('FA-54 a refused create shows the API message and keeps the form', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/permissions');
    await page.locator('#admin-permission-create-node').fill('Bad Node');
    await page.locator('#admin-permission-create-submit').click();
    await expect(error(page)).toContainText('Nodes are lower-case words');
    await expect(page.locator('#admin-permission-create-node')).toHaveValue('Bad Node');
  });

  test('FA-55 deleting a permission asks first, deletes it by its exact ID and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    const messages = [];
    page.on('dialog', (dialog) => {
      messages.push(dialog.message());
      return messages.length === 1 ? dialog.dismiss() : dialog.accept();
    });
    await page.goto('/admin/permissions');
    await permissionRows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect.poll(() => messages.length).toBe(1);
    expect(await writes()).toEqual([]);

    await permissionRows(page).nth(4).getByRole('button', { name: 'Delete' }).click();
    await expect(permissionRows(page)).toHaveCount(4);
    expect(messages).toEqual(['Delete the permission datastore.admin?', 'Delete the permission datastore.admin?']);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/permissions/${ID.pStore}`, body: null }]);
    await expect(page.locator('#admin-permissions-title')).toBeFocused();
  });
});

test.describe('dashboard', () => {
  const cards = (page) => page.locator('a[id$="-link"]');
  const denied = (page) => page.locator('#admin-denied');

  test('FA-56 an account with both admin permissions sees the Users, Roles and Permissions cards, each linking to its page', async ({ page }) => {
    await signIn(page, { me: ['users.admin', 'roles.admin'] });
    await page.goto('/admin');
    await expect(cards(page)).toHaveCount(3);
    await expect(page.locator('#admin-users-link')).toContainText('Users');
    await expect(page.locator('#admin-users-link')).toHaveAttribute('href', '/admin/users');
    await expect(page.locator('#admin-roles-link')).toContainText('Roles');
    await expect(page.locator('#admin-roles-link')).toHaveAttribute('href', '/admin/roles');
    await expect(page.locator('#admin-permissions-link')).toContainText('Permissions');
    await expect(page.locator('#admin-permissions-link')).toHaveAttribute('href', '/admin/permissions');
    await expect(denied(page)).toHaveCount(0);
  });

  for (const [card, path, title] of [
    ['users', '/admin/users', 'Users'],
    ['roles', '/admin/roles', 'Roles'],
    ['permissions', '/admin/permissions', 'Permissions'],
  ]) {
    test(`FA-57 a click on the ${card} card opens its page`, async ({ page }) => {
      await signIn(page, { me: ['users.admin', 'roles.admin'] });
      await page.goto('/admin');
      await page.locator(`#admin-${card}-link`).click();
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
    });
  }

  test('FA-58 an account with only the users permission sees only the Users card', async ({ page }) => {
    await signIn(page, { me: ['users.admin'] });
    await page.goto('/admin');
    await expect(cards(page)).toHaveCount(1);
    await expect(page.locator('#admin-users-link')).toHaveAttribute('href', '/admin/users');
    await expect(page.locator('#admin-roles-link')).toHaveCount(0);
    await expect(page.locator('#admin-permissions-link')).toHaveCount(0);
    await expect(denied(page)).toHaveCount(0);
  });

  test('FA-59 an account with only the roles permission sees the Roles and Permissions cards', async ({ page }) => {
    await signIn(page, { me: ['roles.admin'] });
    await page.goto('/admin');
    await expect(cards(page)).toHaveCount(2);
    await expect(page.locator('#admin-roles-link')).toHaveAttribute('href', '/admin/roles');
    await expect(page.locator('#admin-permissions-link')).toHaveAttribute('href', '/admin/permissions');
    await expect(page.locator('#admin-users-link')).toHaveCount(0);
    await expect(denied(page)).toHaveCount(0);
  });

  test('FA-60 an account with no admin permission sees a notice and no card', async ({ page }) => {
    await signIn(page, { me: [] });
    await page.goto('/admin');
    await expect(denied(page)).toHaveText('Your account has no admin permissions');
    await expect(cards(page)).toHaveCount(0);
  });

  test('FA-61 a failed cards request shows the page error banner and no card', async ({ page }) => {
    await signIn(page, { failures: { 'GET /users/me/permissions': { status: 500, detail: 'permissions are down', times: 1 } } });
    await page.goto('/admin');
    await expect(error(page)).toHaveText('permissions are down');
    await expect(cards(page)).toHaveCount(0);
    await expect(denied(page)).toHaveCount(0);
  });
});
