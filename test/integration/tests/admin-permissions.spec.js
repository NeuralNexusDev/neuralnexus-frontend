import { test, expect, signIn, ID, HOSTILE, error, expectNoInjection } from './helpers.js';

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

  test('API text is rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, { permissions: [{ id: ID.pRate, node: HOSTILE, description: HOSTILE, value_type: 'int', merge: HOSTILE }] });
    await page.goto('/admin/permissions');
    await expect(rows(page).first()).toContainText(HOSTILE);
    await expectNoInjection(page);
  });

});
