import { test, expect, signIn, ID, HOSTILE, error, expectNoInjection } from './helpers.js';

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

  test('saving with Enter in the name field leaves the saved name in the field', async ({ page }) => {
    await signIn(page);
    await page.goto(editor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-name').press('Enter');
    await expect(page.locator('#admin-role-status')).toHaveText('Saved');
    await expect(page.locator('#admin-role-name')).toHaveValue('bee_manager');
    await expect(page.locator('#admin-role-name')).toBeFocused();
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

  test('changing an int value puts the whole number and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await granted(page).nth(1).locator('input').fill('250');
    await granted(page).nth(1).getByRole('button', { name: 'Save value' }).click();
    await expect(granted(page).nth(1).locator('input')).toHaveValue('250');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pRate}`, body: { value: 250 } }]);
    await expect(heading(page)).toBeFocused();
  });

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

  test('removing a permission deletes the grant by its exact ID and moves focus to the list heading', async ({ page }) => {
    const { writes } = await signIn(page);
    await page.goto(editor);
    await granted(page).nth(0).getByRole('button', { name: 'Remove' }).click();
    await expect(granted(page)).toHaveCount(1);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
    await expect(page.locator('#admin-role-grant-permission option')).toContainText(['beenamegenerator.admin']);
    await expect(heading(page)).toBeFocused();
  });

  test('a removal whose editor could not be loaded again offers the permission to grant again', async ({ page }) => {
    await signIn(page, { failures: { [`GET /roles/${ID.bee}`]: { status: 500, detail: 'roles are down', skip: 1, times: 1 } } });
    await page.goto(editor);
    await page.getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(error(page)).toHaveText('The change was made, but the page could not be refreshed: roles are down');
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission option')).toHaveText(['beenamegenerator.admin', 'petpictures.pets', 'motd', 'datastore.admin']);
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

test.describe('admin - role changes while other edits are open', () => {
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

  test('the editor looks busy while a change is in flight and drops a second one', async ({ page }) => {
    const { writes } = await signIn(page, { delays: { [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: 600 } });
    await page.goto(roleEditor);
    await grantedRows(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(page.locator('#admin-role.htmx-request')).toHaveCount(1);
    await expect(page.locator('#admin-role')).toHaveCSS('pointer-events', 'none');
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await grantedRows(page).nth(1).getByRole('button', { name: 'Remove ratelimit' }).dispatchEvent('click');
    await expect(error(page)).toHaveText('The last change is still being saved. Try again in a moment.');
    await expect(grantedRows(page)).toHaveCount(1);
    await expect(page.locator('#admin-role.htmx-request')).toHaveCount(0);
    await expect(page.locator('#admin-role')).not.toHaveAttribute('aria-busy', 'true');
    await expect(error(page)).toHaveText('The change made while saving was not sent. Make it again.');
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
  });

  test('pressing Enter in a value field while another change is in flight says it was dropped', async ({ page }) => {
    const { writes } = await signIn(page, { delays: { [`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`]: 600 } });
    await page.goto(roleEditor);
    await grantedRows(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' }).click();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await page.getByRole('spinbutton', { name: 'Value of ratelimit' }).fill('250');
    await page.getByRole('spinbutton', { name: 'Value of ratelimit' }).press('Enter');
    await expect(error(page)).toHaveText('The last change is still being saved. Try again in a moment.');
    await expect(grantedRows(page)).toHaveCount(1);
    await expect(page.getByRole('spinbutton', { name: 'Value of ratelimit' })).toHaveValue('250');
    await expect(error(page)).toHaveText('The change made while saving was not sent. Make it again.');
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
  });

  test('a grant value select that times out shows the banner message', async ({ page }) => {
    await signIn(page, { delays: { 'GET /permissions': 1500 } });
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-grant-permission')).toBeVisible();
    await page.evaluate(() => {
      htmx.config.defaultTimeout = 300;
    });
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
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
});
