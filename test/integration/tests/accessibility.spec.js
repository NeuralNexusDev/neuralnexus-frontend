import { test, expect, signIn, ID, error, computed, rgba, blend, contrastRatio } from './helpers.js';

for (const colorScheme of ['light', 'dark']) {
  test.describe(`accessibility - contrast in the ${colorScheme} theme`, () => {
    test.use({ colorScheme });

    const pageBackground = async (page) => (await rgba(page, await computed(page.locator('body'), 'backgroundColor'))).slice(0, 3);

    test('a form control border is at least 3:1 against the page', async ({ page }) => {
      await signIn(page);
      await page.goto('/admin/roles');
      const input = page.locator('#admin-role-create-name');
      await expect(input).toBeVisible();
      const border = await rgba(page, await computed(input, 'borderTopColor'));
      const ratio = contrastRatio(blend(border, await pageBackground(page)), await pageBackground(page));
      expect(ratio).toBeGreaterThanOrEqual(3);
    });

    test('the error banner text is at least 4.5:1 against its own background', async ({ page }) => {
      await signIn(page);
      await page.goto('/admin/roles');
      await expect(page.locator('#admin-role-create-name')).toBeVisible();
      await page.evaluate(() => {
        document.getElementById('page-error').textContent = 'Something went wrong';
      });
      const banner = page.locator('#page-error');
      const page_ = await pageBackground(page);
      const background = blend(await rgba(page, await computed(banner, 'backgroundColor')), page_);
      const text = await rgba(page, await computed(banner, 'color'));
      expect(contrastRatio(blend(text, background), background)).toBeGreaterThanOrEqual(4.5);
      const border = await rgba(page, await computed(banner, 'borderTopColor'));
      expect(contrastRatio(blend(border, page_), page_)).toBeGreaterThanOrEqual(3);
    });

    test('the off track of a switch is at least 3:1 against the page', async ({ page }) => {
      await signIn(page, { account: { username: 'testuser', password_auth: false } });
      await page.goto('/account');
      const track = page.locator('label:has(#password-auth-enabled)');
      await expect(track).toBeVisible();
      const background = blend(await rgba(page, await computed(track, 'backgroundColor')), await pageBackground(page));
      expect(contrastRatio(background, await pageBackground(page))).toBeGreaterThanOrEqual(3);
    });
  });
}

test.describe('accessibility - switches', () => {
  test('a switch is a role=switch, at least 24 px tall, and shows a focus ring on its track from the keyboard', async ({ page }) => {
    await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    const toggle = page.getByRole('switch', { name: 'Password login' });
    await expect(toggle).toBeChecked();
    const track = page.locator('label:has(#password-auth-enabled)');
    const box = await track.boundingBox();
    expect(box.height).toBeGreaterThanOrEqual(24);
    expect(box.width).toBeGreaterThanOrEqual(24);
    expect(await computed(track, 'boxShadow')).toBe('none');
    await page.keyboard.press('Tab');
    await toggle.focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    await expect(toggle).toBeFocused();
    expect(await computed(track, 'boxShadow')).not.toBe('none');
  });
});

test.describe('accessibility - page structure', () => {
  test('every shell has its own title', async ({ page }) => {
    await signIn(page);
    for (const [path, title] of [
      ['/admin', 'Admin - NeuralNexus'],
      ['/admin/users', 'Users - NeuralNexus'],
      [`/admin/users/${ID.bob}`, 'Edit user - NeuralNexus'],
      ['/admin/roles', 'Roles - NeuralNexus'],
      [`/admin/roles/${ID.bee}`, 'Edit role - NeuralNexus'],
      ['/admin/permissions', 'Permissions - NeuralNexus'],
      ['/account', 'Account - NeuralNexus'],
    ]) {
      await page.goto(path);
      await expect(page).toHaveTitle(title);
    }
  });

  test('a skip link is hidden until it has focus and moves to the main landmark', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin');
    const skip = page.getByRole('link', { name: 'Skip to main content' });
    expect((await skip.boundingBox()).width).toBeLessThanOrEqual(1);
    await page.keyboard.press('Tab');
    await expect(skip).toBeFocused();
    expect((await skip.boundingBox()).width).toBeGreaterThan(50);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/#main$/);
    await expect(page.getByRole('main')).toHaveCount(1);
    await expect(page.getByRole('banner')).toHaveCount(1);
    await expect(page.getByRole('navigation', { name: 'Main' })).toBeVisible();
  });

  test('the empty error banner takes no room', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-role-create-name')).toBeVisible();
    const box = await page.locator('#page-error').boundingBox();
    expect(box.height).toBe(0);
    expect(await computed(page.locator('#page-error'), 'marginBottom')).toBe('0px');
    await expect(page.locator('#page-error')).toHaveAttribute('role', 'alert');
  });

  test('a shell announces that its content is loading and then loaded through a live region that is already on the page', async ({ page }) => {
    await signIn(page, { delays: { 'GET /roles': 600 } });
    await page.addInitScript(() => {
      window.__announced = [];
      document.addEventListener('DOMContentLoaded', () => {
        new MutationObserver(() => window.__announced.push(document.getElementById('page-status').textContent)).observe(document.getElementById('page-status'), { childList: true, characterData: true, subtree: true });
      });
    });
    await page.goto('/admin/roles');
    await expect(page.locator('#admin-roles li').first()).toBeVisible();
    await expect(page.locator('#page-status')).toHaveText('Loaded');
    expect(await page.evaluate(() => window.__announced)).toEqual(['Loading…', 'Loaded']);
  });
});

test.describe('accessibility - fields', () => {
  test('help text is tied to its input by aria-describedby', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    const input = page.locator('#admin-role-create-name');
    await expect(input).toHaveAttribute('aria-describedby', 'admin-role-create-name-help');
    await expect(page.locator('#admin-role-create-name-help')).toContainText('Lower-case letters');
    await expect(input).not.toHaveAttribute('aria-invalid', 'true');
  });

  test('a role the API refuses is flagged on its name field with the message and focus, and the banner keeps it too', async ({ page }) => {
    await signIn(page, { failures: { 'POST /roles': { status: 409, detail: 'A role with this name already exists', times: 1 } } });
    await page.goto('/admin/roles');
    const input = page.locator('#admin-role-create-name');
    await input.fill('moderator');
    await page.locator('#admin-role-create-submit').click();
    await expect(error(page)).toHaveText('A role with this name already exists');
    await expect(input).toHaveAttribute('aria-invalid', 'true');
    await expect(input).toHaveAttribute('aria-describedby', 'admin-role-create-name-error admin-role-create-name-help');
    await expect(page.locator('#admin-role-create-name-error')).toHaveText('A role with this name already exists');
    await expect(input).toBeFocused();
    await expect(input).toHaveValue('moderator');
    await input.pressSequentially('2');
    await expect(input).not.toHaveAttribute('aria-invalid', 'true');
    await expect(page.locator('#admin-role-create-name-error')).toHaveCount(0);
    await expect(input).toHaveAttribute('aria-describedby', 'admin-role-create-name-help');
  });

  test('a permission the API refuses is flagged on its node field', async ({ page }) => {
    await signIn(page, { failures: { 'POST /permissions': { status: 409, detail: 'A permission with this node already exists', times: 1 } } });
    await page.goto('/admin/permissions');
    const input = page.locator('#admin-permission-create-node');
    await input.fill('pets.write');
    await page.locator('#admin-permission-create-submit').click();
    await expect(error(page)).toHaveText('A permission with this node already exists');
    await expect(input).toHaveAttribute('aria-invalid', 'true');
    await expect(page.locator('#admin-permission-create-node-error')).toHaveText('A permission with this node already exists');
    await expect(input).toBeFocused();
    await expect(input).toHaveValue('pets.write');
  });

  test('an emptied username and a taken username are flagged on the username field', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/users/${ID.bob}`);
    const input = page.locator('#admin-user-username');
    await input.fill('');
    await input.press('Enter');
    await expect(page.locator('#admin-user-username-error')).toHaveText('Enter a username');
    await expect(input).toHaveAttribute('aria-invalid', 'true');
    await expect(input).toBeFocused();
    await input.fill('alice');
    await expect(page.locator('#admin-user-username-error')).toHaveCount(0);
    await input.press('Enter');
    await expect(page.locator('#admin-user-username-error')).toHaveText('An account with this username already exists');
    await expect(input).toHaveAttribute('aria-describedby', 'admin-user-username-error');
    await expect(input).toBeFocused();
    await expect(input).toHaveValue('alice');
  });
});

test.describe('accessibility - live regions', () => {
  test('search and Load more announce a count in a status line that is on the page from the start', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.goto('/admin/users');
    const count = page.locator('#admin-users-count');
    await expect(count).toHaveAttribute('role', 'status');
    await expect(count).toHaveText('200 users');
    await page.locator('#admin-users-more-button').click();
    await expect(count).toHaveText('50 more users');
    await page.locator('#admin-users-search').fill('user-3');
    await expect(count).not.toHaveText('50 more users');
    await page.locator('#admin-users-search').fill('zzzzzz');
    await expect(count).toHaveText(/^No matches/);
  });

  test('the Saved status goes away when the form is edited again', async ({ page }) => {
    await signIn(page);
    await page.goto(`/admin/users/${ID.bob}`);
    const username = page.locator('#admin-user-username');
    await username.fill('robert');
    await page.locator('#admin-user-save').click();
    await expect(page.locator('#admin-user-status')).toHaveText('Saved');
    await username.pressSequentially('x');
    await expect(page.locator('#admin-user-status')).toHaveText('');
  });
});
