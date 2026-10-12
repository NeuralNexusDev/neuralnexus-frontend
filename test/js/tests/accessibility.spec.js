import { test, expect, signIn, error } from './helpers.js';

const schemes = ['light', 'dark'];

/** Resolves any CSS color the browser computes, including oklch and color-mix results, to [r, g, b, a] with a in 0 to 1. */
async function rgba(page, cssColor) {
  return page.evaluate((value) => {
    const context = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = value;
    context.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = context.getImageData(0, 0, 1, 1).data;
    return [r, g, b, a / 255];
  }, cssColor);
}

async function computed(locator, property) {
  return locator.evaluate((element, name) => getComputedStyle(element)[name], property);
}

function luminance([r, g, b]) {
  const [lr, lg, lb] = [r, g, b].map((channel) => {
    const value = channel / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * lr + 0.7152 * lg + 0.0722 * lb;
}

function blend([r, g, b, a], [br, bg, bb]) {
  return [r * a + br * (1 - a), g * a + bg * (1 - a), b * a + bb * (1 - a)];
}

function contrastRatio(foreground, background) {
  const [light, dark] = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (light + 0.05) / (dark + 0.05);
}

const pageBackground = async (page) => (await rgba(page, await computed(page.locator('body'), 'backgroundColor'))).slice(0, 3);

/** The colour of the outermost ring in the box shadow, which is where the focus ring ends. */
async function ringColour(page, locator) {
  const rings = [...(await computed(locator, 'boxShadow')).matchAll(/(rgba?\([^)]*\))\s+0px 0px 0px (\d+)px/g)].sort((a, b) => b[2] - a[2]);
  return rgba(page, rings[0][1]);
}

const roleName = (page) => page.locator('#admin-role-create-name');

async function refuseRole(page) {
  await signIn(page, { failures: { 'POST /roles': { status: 409, detail: 'A role with this name already exists', times: 1 } } });
  await page.goto('/admin/roles');
  await roleName(page).fill('moderator');
  await page.locator('#admin-role-create-submit').click();
  await expect(error(page)).toHaveText('A role with this name already exists');
}

test.describe('contrast', () => {
  for (const colorScheme of schemes) {
    test(`AX-01 a form control border is at least 3:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await signIn(page);
      await page.goto('/admin/roles');
      await expect(roleName(page)).toBeVisible();
      const background = await pageBackground(page);
      const border = await rgba(page, await computed(roleName(page), 'borderTopColor'));
      expect(contrastRatio(blend(border, background), background)).toBeGreaterThanOrEqual(3);
    });
  }

  for (const colorScheme of schemes) {
    test(`AX-02 the error banner text is at least 4.5:1 against its own background and its border 3:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await signIn(page);
      await page.goto('/admin/roles');
      await expect(roleName(page)).toBeVisible();
      await page.evaluate(() => {
        document.getElementById('page-error').textContent = 'Something went wrong';
      });
      const banner = error(page);
      const background = await pageBackground(page);
      const fill = blend(await rgba(page, await computed(banner, 'backgroundColor')), background);
      const text = await rgba(page, await computed(banner, 'color'));
      expect(contrastRatio(blend(text, fill), fill)).toBeGreaterThanOrEqual(4.5);
      const border = await rgba(page, await computed(banner, 'borderTopColor'));
      expect(contrastRatio(blend(border, background), background)).toBeGreaterThanOrEqual(3);
    });
  }

  for (const colorScheme of schemes) {
    test(`AX-03 the off track of a switch is at least 3:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await signIn(page, { account: { username: 'testuser', password_auth: false } });
      await page.goto('/account');
      const track = page.locator('label:has(#password-auth-enabled)');
      await expect(track).toBeVisible();
      const background = await pageBackground(page);
      const fill = blend(await rgba(page, await computed(track, 'backgroundColor')), background);
      expect(contrastRatio(fill, background)).toBeGreaterThanOrEqual(3);
    });
  }

  for (const colorScheme of schemes) {
    test(`AX-04 the muted description of a page is at least 4.5:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await signIn(page);
      await page.goto('/admin/roles');
      const description = page.locator('h1 + p').first();
      await expect(description).toBeVisible();
      const background = await pageBackground(page);
      const text = await rgba(page, await computed(description, 'color'));
      expect(contrastRatio(blend(text, background), background)).toBeGreaterThanOrEqual(4.5);
    });
  }

  for (const colorScheme of schemes) {
    test(`AX-05 the message under a refused field is at least 4.5:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await refuseRole(page);
      const message = page.locator('#admin-role-create-name-error');
      await expect(message).toBeVisible();
      const background = await pageBackground(page);
      const text = await rgba(page, await computed(message, 'color'));
      expect(contrastRatio(blend(text, background), background)).toBeGreaterThanOrEqual(4.5);
    });
  }
});

test.describe('header control contrast', () => {
  const RED_600 = 'oklch(0.577 0.245 27.325)';
  const RED_700 = 'oklch(0.505 0.213 27.518)';
  const controls = [
    { name: 'light/dark toggle', minimum: 3, signedIn: false, locator: (page) => page.locator('button[onclick="toggleTheme()"]') },
    { name: 'Login', minimum: 4.5, signedIn: false, locator: (page) => page.locator('#header-login-btn a') },
    { name: 'Logout', minimum: 4.5, signedIn: true, locator: (page) => page.locator('#header-logout-btn button') },
    { name: 'gear', minimum: 3, signedIn: true, locator: (page) => page.locator('#header-account a[href="/account"]') },
  ];

  const ratio = async (page, locator) => {
    const fill = (await rgba(page, await computed(locator, 'backgroundColor'))).slice(0, 3);
    const text = await rgba(page, await computed(locator, 'color'));
    return contrastRatio(blend(text, fill), fill);
  };

  for (const colorScheme of schemes) {
    for (const control of controls) {
      test(`AX-19 the ${control.name} text or icon is at least ${control.minimum}:1 against red-600 at rest in the ${colorScheme} theme`, async ({ page }) => {
        await page.emulateMedia({ colorScheme });
        await page.addInitScript((theme) => localStorage.setItem('theme', theme), colorScheme);
        await page.route(`${process.env.NN_API_URL}/api/v1/users/me`, (route) =>
          control.signedIn ? route.fulfill({ contentType: 'application/json', body: JSON.stringify({ username: 'alice' }) }) : route.fulfill({ status: 401 }),
        );
        await page.goto('/');
        const target = control.locator(page);
        await expect(target).toBeVisible();
        await expect.poll(() => computed(target, 'backgroundColor')).toBe(RED_600);
        expect(await ratio(page, target)).toBeGreaterThanOrEqual(control.minimum);
      });

      test(`AX-20 the ${control.name} text or icon is at least ${control.minimum}:1 against red-700 on hover in the ${colorScheme} theme`, async ({ page }) => {
        await page.emulateMedia({ colorScheme });
        await page.addInitScript((theme) => localStorage.setItem('theme', theme), colorScheme);
        await page.route(`${process.env.NN_API_URL}/api/v1/users/me`, (route) =>
          control.signedIn ? route.fulfill({ contentType: 'application/json', body: JSON.stringify({ username: 'alice' }) }) : route.fulfill({ status: 401 }),
        );
        await page.goto('/');
        const target = control.locator(page);
        await expect(target).toBeVisible();
        await target.hover();
        await expect.poll(() => computed(target, 'backgroundColor')).toBe(RED_700);
        expect(await ratio(page, target)).toBeGreaterThanOrEqual(control.minimum);
      });
    }
  }
});

test.describe('focus ring', () => {
  for (const colorScheme of schemes) {
    test(`AX-06 a text field and a button show a focus ring from the keyboard and not from the mouse, at least 3:1 against the page in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await signIn(page);
      await page.goto('/admin/roles');
      const submit = page.locator('#admin-role-create-submit');
      await expect(roleName(page)).toBeVisible();
      expect(await computed(roleName(page), 'boxShadow')).toBe('none');
      expect(await computed(submit, 'boxShadow')).toBe('none');
      await submit.hover();
      await page.mouse.down();
      await expect(submit).toBeFocused();
      expect(await computed(submit, 'boxShadow')).toBe('none');
      await page.mouse.move(0, 0);
      await page.mouse.up();
      await page.keyboard.press('Tab');
      await submit.focus();
      const background = await pageBackground(page);
      for (const control of [roleName(page), submit]) {
        await control.focus();
        await expect(control).toBeFocused();
        expect(contrastRatio(blend(await ringColour(page, control), background), background)).toBeGreaterThanOrEqual(3);
      }
    });
  }

  test('AX-07 a switch shows a focus ring on its track from the keyboard', async ({ page }) => {
    await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    const toggle = page.getByRole('switch', { name: 'Password login' });
    await expect(toggle).toBeChecked();
    const track = page.locator('label:has(#password-auth-enabled)');
    expect(await computed(track, 'boxShadow')).toBe('none');
    await page.keyboard.press('Tab');
    await toggle.focus();
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Tab');
    await expect(toggle).toBeFocused();
    expect(await computed(track, 'boxShadow')).not.toBe('none');
  });
});

test.describe('target size', () => {
  test('AX-08 a switch is at least 24 px tall and wide', async ({ page }) => {
    await signIn(page, { account: { username: 'testuser', password_auth: true } });
    await page.goto('/account');
    const track = page.locator('label:has(#password-auth-enabled)');
    await expect(track).toBeVisible();
    const box = await track.boundingBox();
    expect(box.height).toBeGreaterThanOrEqual(24);
    expect(box.width).toBeGreaterThanOrEqual(24);
  });

  test('AX-09 the Link and Unlink buttons are at least 24 px both ways, with a negative margin that keeps the row layout', async ({ page }) => {
    await signIn(page, {
      account: { username: 'testuser', password_auth: true },
      myLinks: [
        { platform: 'discord', platform_username: 'someone#1234', verified: true, login_enabled: true },
        { platform: 'twitch', platform_username: 'streamer99', verified: false, login_enabled: false },
      ],
    });
    await page.goto('/account');
    for (const platform of ['discord', 'twitch', 'microsoft', 'xboxlive', 'steam']) {
      const button = page.locator(`#link-${platform}-action`);
      await expect(button).toBeVisible();
      const box = await button.boundingBox();
      expect(box.width, `${platform} width`).toBeGreaterThanOrEqual(24);
      expect(box.height, `${platform} height`).toBeGreaterThanOrEqual(24);
      expect(await computed(button, 'marginTop'), `${platform} margin`).toBe('-4px');
    }
  });
});

test.describe('page structure', () => {
  test('AX-10 the skip link is hidden until it has focus and moves focus to the main landmark', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin');
    const skip = page.getByRole('link', { name: 'Skip to main content' });
    expect((await skip.boundingBox()).width).toBeLessThanOrEqual(1);
    await page.keyboard.press('Tab');
    await expect(skip).toBeFocused();
    expect((await skip.boundingBox()).width).toBeGreaterThan(50);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/#main$/);
    await expect(page.getByRole('main')).toBeFocused();
    expect(await computed(page.getByRole('main'), 'outlineStyle')).toBe('none');
  });

  test('AX-11 the empty error banner takes no room', async ({ page }) => {
    await signIn(page);
    await page.goto('/admin/roles');
    await expect(roleName(page)).toBeVisible();
    const box = await error(page).boundingBox();
    expect(box.height).toBe(0);
    expect(await computed(error(page), 'marginBottom')).toBe('0px');
  });
});

test.describe('header names', () => {
  for (const colorScheme of schemes) {
    test(`AX-21 the header names the toggle and the gear by their own labels and announces no icon id in the ${colorScheme} theme`, async ({ page }) => {
      await page.emulateMedia({ colorScheme });
      await page.addInitScript((theme) => localStorage.setItem('theme', theme), colorScheme);
      await page.route(`${process.env.NN_API_URL}/api/v1/users/me`, (route) =>
        route.fulfill({ contentType: 'application/json', body: JSON.stringify({ username: 'alice' }) }),
      );
      await page.goto('/');
      const header = page.locator('header');
      const gear = header.getByRole('link', { name: 'Account settings' });
      await expect(gear).toHaveAttribute('href', '/account');
      await expect(header.getByRole('button', { name: 'Toggle dark mode' })).toHaveCount(1);
      await expect(header.getByRole('img')).toHaveCount(0);
      expect(await header.ariaSnapshot()).not.toContain('material-symbols');
    });
  }
});

test.describe('live regions', () => {
  test('AX-12 a shell announces that its content is loading and then loaded through a live region that is already on the page', async ({ page }) => {
    const { gate } = await signIn(page);
    const listed = await gate('GET /roles');
    await page.addInitScript(() => {
      window.__announced = [];
      document.addEventListener('DOMContentLoaded', () => {
        new MutationObserver(() => window.__announced.push(document.getElementById('page-status').textContent)).observe(document.getElementById('page-status'), { childList: true, characterData: true, subtree: true });
      });
    });
    await page.goto('/admin/roles');
    await listed.arrived();
    await expect(page.locator('#page-status')).toHaveText('Loading…');
    await listed.release();
    await expect(page.locator('#admin-roles li').first()).toBeVisible();
    await expect(page.locator('#page-status')).toHaveText('Loaded');
    expect(await page.evaluate(() => window.__announced)).toEqual(['Loading…', 'Loaded']);
  });

  test('AX-14 a shell whose content cannot reach the server drops the Loading line and says so in the live region', async ({ page }) => {
    await signIn(page);
    await page.route('**/admin/roles/list', (route) => route.abort('failed'));
    await page.goto('/admin/roles');
    await expect(page.locator('#page-status')).toHaveText('Failed to load');
    await expect(page.getByText('Loading…')).toHaveCount(0);
  });

  test('AX-15 search and Load more announce a count in a status line that is on the page from the start', async ({ page }) => {
    await signIn(page, { generateUsers: 247 });
    await page.addInitScript(() => {
      document.addEventListener('DOMContentLoaded', () => {
        const line = document.getElementById('admin-users-count');
        window.__countAtLoad = line ? line.textContent : null;
      });
    });
    await page.goto('/admin/users');
    expect(await page.evaluate(() => window.__countAtLoad)).toBe('');
    const count = page.locator('#admin-users-count');
    await expect(count).toHaveText('200 users');
    await page.locator('#admin-users-more-button').click();
    await expect(count).toHaveText('50 more users');
    await page.locator('#admin-users-search').fill('user-3');
    await expect(count).not.toHaveText('50 more users');
    await page.locator('#admin-users-search').fill('zzzzzz');
    await expect(count).toHaveText(/^No matches/);
  });

});

test.describe('fields', () => {
  test('AX-17 a role the API refuses is described on its name field by the message, which keeps focus and the typed value, and the banner keeps it too', async ({ page }) => {
    await refuseRole(page);
    await expect(page.locator('#admin-role-create-name-error')).toHaveText('A role with this name already exists');
    await expect(roleName(page)).toHaveAccessibleDescription(/A role with this name already exists/);
    await expect(roleName(page)).toBeFocused();
    await expect(roleName(page)).toHaveValue('moderator');
  });

  test('AX-18 editing a refused field clears its flag and its message', async ({ page }) => {
    await refuseRole(page);
    await expect(roleName(page)).toHaveAttribute('aria-invalid', 'true');
    await roleName(page).pressSequentially('2');
    await expect(roleName(page)).not.toHaveAttribute('aria-invalid', 'true');
    await expect(page.locator('#admin-role-create-name-error')).toHaveCount(0);
    await expect(roleName(page)).not.toHaveAccessibleDescription(/A role with this name already exists/);
    await expect(roleName(page)).toHaveAccessibleDescription(/Lower-case letters, digits and underscores/);
  });
});
