import { test, expect, signIn, HOSTILE, error, expectNoInjection } from './helpers.js';

test.describe('bee name generator - suggestion review', () => {
  const rows = (page) => page.locator('#bee-suggestions li');
  const page_ = '/project/bee-name-generator/admin';

  test('lists the pending suggestions', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto(page_);
    await expect(rows(page)).toHaveText([/buzz/, /honey/, /wax/]);
  });

  test('accepting sends a PUT for the exact name and removes the row', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Accept honey' }).click();
    await expect(rows(page)).toHaveText([/buzz/, /wax/]);
    expect(await writes()).toEqual([{ method: 'PUT', path: '/bee-name-generator/suggestion/honey', body: null }]);
    await expect(page.locator('#bee-suggestions-title')).toBeFocused();
  });

  test('rejecting sends a DELETE for a name that needs escaping', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['beenamegenerator.admin'], suggestions: ['royal jelly/queen?'] });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Reject royal jelly/queen?' }).click();
    await expect(page.locator('#bee-suggestions-empty')).toHaveText('No pending suggestions');
    expect(await writes()).toEqual([{ method: 'DELETE', path: '/bee-name-generator/suggestion/royal%20jelly%2Fqueen%3F', body: null }]);
  });

  test('a refused review shows the API message and keeps the row', async ({ page }) => {
    await signIn(page, {
      me: ['beenamegenerator.admin'],
      failures: { 'PUT /bee-name-generator/suggestion/buzz': { status: 500, detail: 'Failed to accept', times: 1 } },
    });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(error(page)).toHaveText('Failed to accept');
    await expect(rows(page)).toHaveCount(3);

    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(rows(page)).toHaveCount(2);
    await expect(error(page)).toHaveText('');
  });

  test('a second review while one is in flight is dropped', async ({ page }) => {
    const { writes } = await signIn(page, {
      me: ['beenamegenerator.admin'],
      delays: { 'PUT /bee-name-generator/suggestion/buzz': 600 },
    });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(page.locator('#bee-suggestions-root[aria-busy="true"]')).toHaveCount(1);
    await page.getByRole('button', { name: 'Reject honey' }).dispatchEvent('click');
    await expect(rows(page)).toHaveText([/honey/, /wax/]);
    expect(await writes()).toEqual([{ method: 'PUT', path: '/bee-name-generator/suggestion/buzz', body: null }]);
  });

  test('suggestions are rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'], suggestions: [HOSTILE] });
    await page.goto(page_);
    await expect(rows(page)).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});
