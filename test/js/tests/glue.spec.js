import { test, expect, signIn, ID, error } from './helpers.js';

const roleEditor = `/admin/roles/${ID.bee}`;
const userEditor = `/admin/users/${ID.bob}`;

const granted = (page) => page.locator('#admin-role-permissions li');
const valueField = (page, label) => page.getByRole('spinbutton', { name: `Value of ${label}` });
const userRows = (page) => page.locator('#admin-users > li:has(> a)');
const search = (page) => page.locator('#admin-users-search');
const username = (page) => page.locator('#admin-user-username');
const userStatus = (page) => page.locator('#admin-user-status');
const roleStatus = (page) => page.locator('#admin-role-status');
const removeFirstGrant = (page) => granted(page).nth(0).getByRole('button', { name: 'Remove beenamegenerator.admin' });
const removal = (gate) => gate(`DELETE /roles/${ID.bee}/permissions/${ID.pBee}`);
const droppedMessage = 'The last change is still being saved. Try again in a moment.';
const notSentMessage = 'The change made while saving was not sent. Make it again.';
const unsavedNote = 'Changes made while saving are not saved yet.';

const grantsWithValues = [
  { id: ID.pBee },
  { id: ID.pRate, value: 100 },
  { id: ID.pMotd, value: 'hi' },
];

test.describe('page status', () => {
  test('HG-01 a page load on a page without a live region throws nothing', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    const thrown = [];
    page.on('pageerror', (failure) => thrown.push(failure.message));
    await page.goto('/project/bee-name-generator');
    await expect(page.locator('#bee-admin-link')).toBeVisible();
    await expect(page.locator('#page-status')).toHaveCount(0);
    expect(thrown).toEqual([]);
  });

  test('HG-02 a request that is not a page load leaves the page status alone', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.addInitScript(() => {
      window.__announced = [];
      document.addEventListener('DOMContentLoaded', () => {
        new MutationObserver(() => window.__announced.push(document.getElementById('page-status').textContent)).observe(document.getElementById('page-status'), { childList: true, characterData: true, subtree: true });
      });
    });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    await expect(page.locator('#page-status')).toHaveText('Loaded');
    const searched = await gate('GET /users');
    await search(page).fill('bob');
    await searched.arrived();
    await expect(page.locator('#page-status')).toHaveText('Loaded');
    await searched.release();
    await expect(userRows(page)).toHaveCount(1);
    await expect(page.locator('#page-status')).toHaveText('Loaded');
    expect(await page.evaluate(() => window.__announced)).toEqual(['Loading…', 'Loaded']);
  });
});

test.describe('busy state', () => {
  test('HG-03 a form dims its button and shows a progress cursor while its request runs', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto('/admin/roles');
    const post = await gate('POST /roles');
    await page.locator('#admin-role-create-name').fill('moderator');
    await page.locator('#admin-role-create-submit').click();
    await post.arrived();
    await expect(page.locator('#admin-role-create-form')).toHaveAttribute('aria-busy', 'true');
    await expect(page.locator('#admin-role-create-form')).toHaveCSS('cursor', 'progress');
    await expect(page.locator('#admin-role-create-submit')).toHaveCSS('opacity', '0.6');
    await post.release();
    await expect(page).toHaveURL(/\/admin\/roles\/\d{19}$/);
  });

  test('HG-04 a region stays busy until the last of its requests has finished', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto(roleEditor);
    const first = await removal(gate);
    await removeFirstGrant(page).click();
    await first.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await expect(page.locator('#admin-role-grant-value input[type="text"]')).toBeVisible();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await valueField(page, 'ratelimit').fill('88');
    await valueField(page, 'ratelimit').press('Enter');
    await expect(error(page)).toHaveText(droppedMessage);
    await first.release();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role')).not.toHaveAttribute('aria-busy', 'true');
  });

  test('HG-05 a search that a newer search replaces shows no error and logs none', async ({ page }) => {
    const { gate } = await signIn(page);
    const logged = [];
    page.on('console', (message) => message.type() === 'error' && message.text().startsWith('htmx') && logged.push(message.text()));
    await page.addInitScript(() => {
      window.__banner = [];
      new MutationObserver(() => {
        const text = document.getElementById('page-error')?.textContent;
        if (text) window.__banner.push(text);
      }).observe(document, { subtree: true, childList: true, characterData: true });
    });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    const searched = await gate('GET /users');
    await search(page).fill('bo');
    await searched.arrived(1);
    await search(page).fill('alice');
    await searched.arrived(2);
    await searched.release();
    await expect(userRows(page)).toHaveCount(1);
    await expect(userRows(page)).toContainText('alice');
    await expect(error(page)).toHaveText('');
    expect(await page.evaluate(() => window.__banner)).toEqual([]);
    expect(logged).toEqual([]);
  });
});

test.describe('late answers', () => {
  test('HG-10 a late answer to an earlier select change does not replace the answer to the last one', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto(roleEditor);
    const select = page.locator('#admin-role-grant-permission');
    await expect(select).toBeVisible();
    const catalogue = await gate('GET /permissions');
    for (const [index, label] of ['datastore.admin', 'petpictures.pets', 'motd'].entries()) {
      await select.selectOption({ label });
      await catalogue.arrived(index + 1);
    }
    await catalogue.release({ index: 2 });
    await expect(page.locator('#admin-role-grant-value input[type="text"]')).toBeVisible();
    const late = page.waitForResponse((response) => response.url().includes(`grant_permission=${ID.pPets}`));
    await catalogue.release({ index: 1 });
    await late;
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await expect(page.locator('#admin-role-grant-value input[type="text"]')).toBeVisible();
    await expect(page.locator('#admin-role-grant-value textarea')).toHaveCount(0);
    await catalogue.release();
  });

  test('HG-11 a late answer to an earlier search does not replace the answer to the last one', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    const searched = await gate('GET /users');
    for (const [index, text] of ['bo', 'nobody', 'alice'].entries()) {
      await search(page).fill(text);
      await searched.arrived(index + 1);
    }
    await searched.release({ index: 2 });
    await expect(userRows(page)).toHaveCount(1);
    await expect(userRows(page)).toContainText('alice');
    const late = page.waitForResponse((response) => response.url().includes('search=nobody'));
    await searched.release({ index: 1 });
    await late;
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await expect(userRows(page)).toHaveCount(1);
    await expect(userRows(page)).toContainText('alice');
    await expect(page.locator('#admin-users-empty')).toHaveCount(0);
    await searched.release();
  });
});

test.describe('values typed while a request runs', () => {
  test('HG-20 the tick made during a save is put back, noted, and sent by the next save', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(userEditor);
    const put = await gate(`PUT /users/${ID.bob}`);
    await username(page).fill('robert');
    await username(page).press('Enter');
    await put.arrived();
    await expect(page.locator('#admin-user')).toHaveAttribute('aria-busy', 'true');
    await page.locator(`#admin-user-roles input[value="${ID.owner}"]`).focus();
    await page.keyboard.press('Space');
    await put.release();
    await expect(userStatus(page)).toHaveText(`Saved. ${unsavedNote}`);
    await page.locator('#admin-user-save').click();
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(page.locator(`#admin-user-roles input[value="${ID.owner}"]`)).toBeChecked();
    expect(await writes()).toEqual([
      { method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } },
      { method: 'PUT', path: `/users/${ID.bob}`, body: { roles: [ID.owner, ID.bee] } },
    ]);
  });

  test('HG-21 a refused save keeps the tick and the text without the note', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto(userEditor);
    const put = await gate(`PUT /users/${ID.bob}`);
    await username(page).fill('alice');
    await username(page).press('Enter');
    await put.arrived();
    await expect(page.locator('#admin-user')).toHaveAttribute('aria-busy', 'true');
    await page.locator(`#admin-user-roles input[value="${ID.owner}"]`).focus();
    await page.keyboard.press('Space');
    await put.release({ status: 409, detail: 'An account with this username already exists' });
    await expect(error(page)).toHaveText('An account with this username already exists');
    await expect(page.locator('#admin-user')).not.toHaveAttribute('aria-busy', 'true');
    await expect(username(page)).toHaveValue('alice');
    await expect(page.locator(`#admin-user-roles input[value="${ID.owner}"]`)).toBeChecked();
    await expect(userStatus(page)).toHaveText('');
  });

  test('HG-22 the caret stays where it was in a text field typed in while a save is in flight', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto(userEditor);
    const put = await gate(`PUT /users/${ID.bob}`);
    await username(page).fill('robert');
    await username(page).press('Enter');
    await put.arrived();
    await expect(page.locator('#admin-user')).toHaveAttribute('aria-busy', 'true');
    await username(page).press('End');
    await username(page).pressSequentially('XY');
    await username(page).press('ArrowLeft');
    await username(page).press('ArrowLeft');
    await put.release();
    await expect(userStatus(page)).toHaveText(`Saved. ${unsavedNote}`);
    await expect(username(page)).toHaveValue('robertXY');
    await expect(username(page)).toBeFocused();
    expect(await username(page).evaluate((input) => input.selectionStart)).toBe(6);
  });

  test('HG-23 a number field typed in while a removal is in flight keeps its value and focus', async ({ page }) => {
    const { gate } = await signIn(page);
    await page.goto(roleEditor);
    const held = await removal(gate);
    await removeFirstGrant(page).click();
    await held.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    const value = valueField(page, 'ratelimit');
    await value.fill('7712');
    await value.press('ArrowLeft');
    await held.release();
    await expect(granted(page)).toHaveCount(1);
    await expect(value).toHaveValue('7712');
    await expect(value).toBeFocused();
  });

  test('HG-24 the field that has focus is the one that gets focus back, not the last one changed', async ({ page }) => {
    const { gate } = await signIn(page, {
      roles: [{ id: ID.bee, name: 'bee_admin', description: 'd', grants: grantsWithValues }],
    });
    await page.goto(roleEditor);
    const held = await removal(gate);
    await removeFirstGrant(page).click();
    await held.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await granted(page).nth(2).locator('input').fill('half typed');
    await valueField(page, 'ratelimit').fill('321');
    await held.release();
    await expect(granted(page)).toHaveCount(2);
    await expect(valueField(page, 'ratelimit')).toHaveValue('321');
    await expect(granted(page).nth(1).locator('input')).toHaveValue('half typed');
    await expect(valueField(page, 'ratelimit')).toBeFocused();
  });

  test('HG-25 a grant form changed while a removal is in flight is dropped as a whole and says so, and another form keeps its value', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const held = await removal(gate);
    await removeFirstGrant(page).click();
    await held.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await valueField(page, 'ratelimit').fill('321');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input[type="text"]').fill('hello');
    await held.release();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role-grant-permission option:checked')).toHaveText('petpictures.pets');
    await expect(page.locator('#admin-role-grant-value textarea')).toHaveValue('');
    await expect(error(page)).toHaveText(notSentMessage);
    await expect(valueField(page, 'ratelimit')).toHaveValue('321');
    await expect(roleStatus(page)).toHaveText(unsavedNote);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
  });

  test('HG-26 a grant form changed while a rename is in flight keeps what was chosen and typed without a message', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const patch = await gate(`PATCH /roles/${ID.bee}`);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-name').press('Enter');
    await patch.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await page.locator('#admin-role-grant-permission').selectOption({ label: 'motd' });
    await page.locator('#admin-role-grant-value input[type="text"]').fill('hello');
    await patch.release();
    await expect(roleStatus(page)).toHaveText('Saved');
    await expect(page.locator('#admin-role-grant-permission option:checked')).toHaveText('motd');
    await expect(page.locator('#admin-role-grant-value input[type="text"]')).toHaveValue('hello');
    await expect(error(page)).toHaveText('');
    await page.locator('#admin-role-grant-submit').click();
    await expect(granted(page)).toHaveCount(3);
    expect(await writes()).toEqual([
      { method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } },
      { method: 'PUT', path: `/roles/${ID.bee}/permissions/${ID.pMotd}`, body: { value: 'hello' } },
    ]);
  });

  test('HG-27 a field outside the redrawn region changed during a rename leaves the status at Saved', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const patch = await gate(`PATCH /roles/${ID.bee}`);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-name').press('Enter');
    await patch.arrived();
    await expect(page.locator('#admin-role')).toHaveAttribute('aria-busy', 'true');
    await valueField(page, 'ratelimit').fill('99');
    await patch.release();
    await expect(page.locator('#admin-role')).not.toHaveAttribute('aria-busy', 'true');
    await expect(roleStatus(page)).toHaveText('Saved');
    await expect(valueField(page, 'ratelimit')).toHaveValue('99');
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
  });
});

test.describe('second action while one is in flight', () => {
  test('HG-30 the region looks busy, takes no pointer input, and drops a second action with a message', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const held = await removal(gate);
    await removeFirstGrant(page).click();
    await held.arrived();
    await expect(page.locator('#admin-role[aria-busy="true"]')).toHaveCount(1);
    await expect(page.locator('#admin-role')).toHaveCSS('pointer-events', 'none');
    await granted(page).nth(1).getByRole('button', { name: 'Remove ratelimit' }).dispatchEvent('click');
    await expect(error(page)).toHaveText(droppedMessage);
    await held.release();
    await expect(granted(page)).toHaveCount(1);
    await expect(page.locator('#admin-role[aria-busy="true"]')).toHaveCount(0);
    await expect(error(page)).toHaveText(notSentMessage);
    expect(await writes()).toEqual([{ method: 'DELETE', path: `/roles/${ID.bee}/permissions/${ID.pBee}`, body: null }]);
  });

  test('HG-31 the same button over the same values dropped a second time sends one write and no note', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const patch = await gate(`PATCH /roles/${ID.bee}`);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await patch.arrived();
    await page.locator('#admin-role-save').dispatchEvent('click');
    await expect(error(page)).toHaveText(droppedMessage);
    await page.locator('#admin-role-save').dispatchEvent('click');
    await patch.release();
    await expect(roleStatus(page)).toHaveText('Saved');
    await expect(error(page)).toHaveText('');
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
  });

  test('HG-32 the same button dropped after a field changed says the change was not sent and puts the text back', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(roleEditor);
    const patch = await gate(`PATCH /roles/${ID.bee}`);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await patch.arrived();
    await page.locator('#admin-role-description').fill('Changed meanwhile');
    await page.locator('#admin-role-save').dispatchEvent('click');
    await expect(error(page)).toHaveText(droppedMessage);
    await patch.release();
    await expect(error(page)).toHaveText(notSentMessage);
    await expect(page.locator('#admin-role-description')).toHaveValue('Changed meanwhile');
    await expect(roleStatus(page)).toHaveText(`Saved. ${unsavedNote}`);
    expect(await writes()).toEqual([{ method: 'PATCH', path: `/roles/${ID.bee}`, body: { name: 'bee_manager' } }]);
  });

  test('HG-33 Enter in the same unchanged field dropped a second time sends one write and no note', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(userEditor);
    const put = await gate(`PUT /users/${ID.bob}`);
    await username(page).fill('robert');
    await username(page).press('Enter');
    await put.arrived();
    await username(page).press('Enter');
    await expect(error(page)).toHaveText(droppedMessage);
    await put.release();
    await expect(userStatus(page)).toHaveText('Saved');
    await expect(error(page)).toHaveText('');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
  });

  test('HG-34 Enter after the field changed says the change was not sent and keeps the text', async ({ page }) => {
    const { writes, gate } = await signIn(page);
    await page.goto(userEditor);
    const put = await gate(`PUT /users/${ID.bob}`);
    await username(page).fill('robert');
    await username(page).press('Enter');
    await put.arrived();
    await expect(page.locator('#admin-user')).toHaveAttribute('aria-busy', 'true');
    await username(page).press('End');
    await username(page).pressSequentially('X');
    await username(page).press('Enter');
    await expect(error(page)).toHaveText(droppedMessage);
    await put.release();
    await expect(userStatus(page)).toHaveText(`Saved. ${unsavedNote}`);
    await expect(error(page)).toHaveText(notSentMessage);
    await expect(username(page)).toHaveValue('robertX');
    expect(await writes()).toEqual([{ method: 'PUT', path: `/users/${ID.bob}`, body: { username: 'robert' } }]);
  });

  test('HG-35 a region marked busy with no request registered still drops the action with the message', async ({ page }) => {
    await signIn(page);
    const thrown = [];
    page.on('pageerror', (failure) => thrown.push(failure.message));
    await page.goto(roleEditor);
    await expect(page.locator('#admin-role-save')).toBeVisible();
    await page.locator('#admin-role').evaluate((region) => {
      region.setAttribute('aria-busy', 'true');
      region.insertAdjacentHTML('beforeend', '<button type="button" id="plain-button">Plain</button>');
    });
    await page.locator('#plain-button').dispatchEvent('click');
    await expect(error(page)).toHaveText(droppedMessage);
    expect(thrown).toEqual([]);
  });

  test('HG-36 a change to a select in a region that is redrawn whole is undone and dropped, and a text field is left alone', async ({ page }) => {
    const { gate } = await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    const region = page.locator('#account-password');
    await expect(page.locator('#password-auth-enabled')).toBeChecked();
    await region.evaluate((element) => {
      element.insertAdjacentHTML('beforeend', '<select name="pick" aria-label="Pick"><option value="a">a</option><option value="b">b</option></select><input type="text" name="note" aria-label="Note" value="n"/>');
    });
    const patch = await gate('PATCH /users/me/settings');
    await page.locator('#password-auth-enabled').focus();
    await page.keyboard.press('Space');
    await patch.arrived();
    await page.getByLabel('Pick').selectOption('b');
    await expect(page.getByLabel('Pick')).toHaveValue('a');
    await expect(error(page)).toHaveText(droppedMessage);
    await page.evaluate(() => {
      document.getElementById('page-error').textContent = '';
    });
    await page.getByLabel('Note').fill('typed');
    await page.getByLabel('Note').blur();
    await expect(page.getByLabel('Note')).toHaveValue('typed');
    await expect(error(page)).toHaveText('');
    await patch.release();
  });
});

test.describe('page load answers', () => {
  test('HG-40 a page load answered with a 4xx status drops the Loading line and says it failed', async ({ page }) => {
    await signIn(page, { failures: { 'GET /roles': { status: 400, detail: 'Bad request', times: 1 } } });
    await page.goto('/admin/roles');
    await expect(error(page)).toHaveText('Bad request');
    await expect(page.getByText('Loading…')).toHaveCount(0);
    await expect(page.locator('#page-status')).toHaveText('Failed to load');
  });

  test('HG-41 a page load answered with 200 whose swap fails drops the Loading line and says it failed', async ({ page }) => {
    await signIn(page);
    await page.route('**/admin/roles/list', (route) => route.fulfill({ status: 200, contentType: 'text/html', headers: { 'hx-reswap': 'bogus' }, body: '<p>hi</p>' }));
    await page.goto('/admin/roles');
    await expect(page.locator('#page-status')).toHaveText('Failed to load');
    await expect(page.getByText('Loading…')).toHaveCount(0);
  });
});

test.describe('fields and status lines', () => {
  test('HG-50 editing a field empties the status line of its region', async ({ page }) => {
    await signIn(page);
    await page.goto(roleEditor);
    await page.locator('#admin-role-name').fill('bee_manager');
    await page.locator('#admin-role-save').click();
    await expect(roleStatus(page)).toHaveText('Saved');
    await page.locator('#admin-role-description').pressSequentially('x');
    await expect(roleStatus(page)).toHaveText('');
  });

  test('HG-51 editing a flagged field removes its flag and message and keeps the other descriptions in order', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    const input = page.locator('#admin-role-create-name');
    await expect(input).toBeVisible();
    await input.evaluate((field) => {
      field.setAttribute('aria-invalid', 'true');
      field.setAttribute('aria-describedby', `${field.id}-error  first second`);
      field.insertAdjacentHTML('afterend', `<p id="${field.id}-error">Refused</p>`);
    });
    await input.pressSequentially('x');
    await expect(input).not.toHaveAttribute('aria-invalid', 'true');
    await expect(page.locator('#admin-role-create-name-error')).toHaveCount(0);
    await expect(input).toHaveAttribute('aria-describedby', 'first second');
  });

  test('HG-52 editing a flagged field that has no description clears the flag and leaves no description', async ({ page }) => {
    await signIn(page);
    const thrown = [];
    page.on('pageerror', (failure) => thrown.push(failure.message));
    await page.goto('/admin/roles');
    const input = page.locator('#admin-role-create-name');
    await expect(input).toBeVisible();
    await input.evaluate((field) => {
      field.setAttribute('aria-invalid', 'true');
      field.removeAttribute('aria-describedby');
    });
    await input.pressSequentially('x');
    await expect(input).not.toHaveAttribute('aria-invalid', 'true');
    await expect(input).not.toHaveAttribute('aria-describedby');
    expect(thrown).toEqual([]);
  });
});

test.describe('messages the glue shows', () => {
  test('HG-60 a search that times out shows the banner message and keeps the list', async ({ page }) => {
    await signIn(page, { delays: { 'GET /users': 1500 } });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    await page.evaluate(() => {
      htmx.config.defaultTimeout = 300;
    });
    await search(page).fill('bob');
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(userRows(page)).toHaveCount(3);
  });

  test('HG-61 a search the browser cannot send shows the banner message and keeps the list', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(3);
    await page.route('**/admin/users/rows*', (route) => route.abort('failed'));
    await search(page).fill('bob');
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(userRows(page)).toHaveCount(3);
  });

  test('HG-62 a message is scrolled into view when the banner is off screen', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    await expect(userRows(page)).toHaveCount(200);
    await page.route('**/admin/users/rows*', (route) => route.abort('failed'));
    await page.locator('#admin-users-more-button').scrollIntoViewIfNeeded();
    await expect(error(page)).not.toBeInViewport();
    await page.locator('#admin-users-more-button').click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(error(page)).toBeInViewport();
  });
});
