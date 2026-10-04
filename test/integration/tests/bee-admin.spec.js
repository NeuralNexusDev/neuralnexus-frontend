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

  test('an empty list says there is nothing to review', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'], suggestions: [] });
    await page.goto(page_);
    await expect(page.locator('#bee-suggestions-empty')).toBeVisible();
    await expect(page.locator('#bee-suggestions')).toHaveCount(0);
  });

  test('an account without the permission sees the API message and no list', async ({ page }) => {
    await signIn(page, { me: [] });
    await page.goto(page_);
    await expect(error(page)).toHaveText('You do not have permission to review suggestions');
    await expect(page.locator('#bee-suggestions-root')).toHaveCount(0);
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

  test('names made only of dots are refused without a request', async ({ page }) => {
    const { writes } = await signIn(page, { me: ['beenamegenerator.admin'], suggestions: ['..', 'ok'] });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Accept ..' }).click();
    await expect(error(page)).toHaveText('That name cannot be reviewed here');
    await expect(rows(page)).toHaveCount(2);
    expect(await writes()).toEqual([]);
  });

  test('a second review while one is in flight is dropped', async ({ page }) => {
    const { writes } = await signIn(page, {
      me: ['beenamegenerator.admin'],
      delays: { 'PUT /bee-name-generator/suggestion/buzz': 600 },
    });
    await page.goto(page_);
    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await page.getByRole('button', { name: 'Reject honey' }).click();
    await expect(rows(page)).toHaveText([/honey/, /wax/]);
    expect(await writes()).toEqual([{ method: 'PUT', path: '/bee-name-generator/suggestion/buzz', body: null }]);
  });

  test('a server that cannot be reached shows a message in the banner', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'] });
    await page.goto(page_);
    await page.route('**/admin/suggestions', (route) => (route.request().method() === 'POST' ? route.abort('failed') : route.continue()));
    await page.getByRole('button', { name: 'Accept buzz' }).click();
    await expect(error(page)).toHaveText('The server could not be reached. Try again in a moment.');
    await expect(rows(page)).toHaveCount(3);
  });

  test('a signed-out visitor is sent to the login page', async ({ page }) => {
    await page.route('**/login', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }));
    await page.goto(page_);
    await expect(page).toHaveURL(/\/login$/);
  });

  test('suggestions are rendered as text, never as HTML', async ({ page }) => {
    await signIn(page, { me: ['beenamegenerator.admin'], suggestions: [HOSTILE] });
    await page.goto(page_);
    await expect(rows(page)).toContainText(HOSTILE);
    await expectNoInjection(page);
  });
});
