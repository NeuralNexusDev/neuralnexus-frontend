import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;

test.beforeEach(async ({ page }) => {
  await page.route(`${API}/users/me`, (route) => route.fulfill({ status: 401 }));
});

const store = (page, value) => page.addInitScript((theme) => localStorage.setItem('theme', theme), value);
const isDark = (page) => page.evaluate(() => document.documentElement.classList.contains('dark'));
const stored = (page) => page.evaluate(() => localStorage.getItem('theme'));
const toggle = (page) => page.locator('button[onclick="toggleTheme()"]');
const lightIcon = (page) => page.locator('#light-mode');
const darkIcon = (page) => page.locator('#dark-mode');

async function expectDark(page) {
  expect(await isDark(page)).toBe(true);
  await expect(lightIcon(page)).toBeVisible();
  await expect(darkIcon(page)).toBeHidden();
}

async function expectLight(page) {
  expect(await isDark(page)).toBe(false);
  await expect(lightIcon(page)).toBeHidden();
  await expect(darkIcon(page)).toBeVisible();
}

test.describe('getThemePreference', () => {
  for (const [saved, scheme] of [
    ['light', 'dark'],
    ['dark', 'light'],
  ]) {
    test(`TH-01 a stored ${saved} wins over a ${scheme} system`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await store(page, saved);
      await page.goto('/');
      expect(await page.evaluate(() => getThemePreference())).toBe(saved);
    });
  }

  for (const scheme of ['light', 'dark']) {
    test(`TH-02 with nothing stored it follows a ${scheme} system`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await page.goto('/');
      expect(await page.evaluate(() => getThemePreference())).toBe(scheme);
    });
  }

  for (const [saved, scheme] of [
    ['system', 'light'],
    ['blue', 'dark'],
  ]) {
    test(`TH-03 a stored ${saved} is not a preference, so it follows a ${scheme} system`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await store(page, saved);
      await page.goto('/');
      expect(await page.evaluate(() => getThemePreference())).toBe(scheme);
    });
  }
});

test.describe('saveThemePreference', () => {
  test('TH-04 it stores the preference under theme and a second call replaces it', async ({ page }) => {
    await page.goto('/');
    expect(await stored(page)).toBeNull();
    await page.evaluate(() => saveThemePreference('dark'));
    expect(await stored(page)).toBe('dark');
    await page.evaluate(() => saveThemePreference('light'));
    expect(await stored(page)).toBe('light');
  });
});

test.describe('toggleTheme', () => {
  test('TH-05 from light it turns dark, shows the light icon and stores dark', async ({ page }) => {
    await store(page, 'light');
    await page.goto('/');
    await expectLight(page);
    await toggle(page).click();
    await expectDark(page);
    expect(await stored(page)).toBe('dark');
  });

  test('TH-06 from dark it turns light, shows the dark icon and stores light', async ({ page }) => {
    await store(page, 'dark');
    await page.goto('/');
    await expectDark(page);
    await toggle(page).click();
    await expectLight(page);
    expect(await stored(page)).toBe('light');
  });

  for (const [scheme, afterDark] of [
    ['light', true],
    ['dark', false],
  ]) {
    test(`TH-07 with nothing stored it turns a ${scheme} system the other way and stores that`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await page.goto('/');
      await toggle(page).click();
      expect(await isDark(page)).toBe(afterDark);
      expect(await stored(page)).toBe(afterDark ? 'dark' : 'light');
    });
  }

  test('TH-08 two toggles end where they began, and the choice survives a reload', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'light' });
    await page.goto('/');
    await toggle(page).click();
    await expectDark(page);
    await page.reload();
    await expectDark(page);
    await toggle(page).click();
    await expectLight(page);
    await page.reload();
    await expectLight(page);
    expect(await stored(page)).toBe('light');
  });

  test('TH-09 the page colours follow the class', async ({ page }) => {
    await store(page, 'light');
    await page.goto('/');
    const background = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    const light = await background();
    await toggle(page).click();
    await expect.poll(background).not.toBe(light);
    await toggle(page).click();
    await expect.poll(background).toBe(light);
  });
});

test.describe('load-time setup', () => {
  test('TH-10 a stored light gives a light page with the dark icon, whatever the system', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
    await store(page, 'light');
    await page.goto('/');
    await expectLight(page);
  });

  test('TH-11 a stored dark gives a dark page with the light icon, whatever the system', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'light' });
    await store(page, 'dark');
    await page.goto('/');
    await expectDark(page);
  });

  for (const scheme of ['light', 'dark']) {
    test(`TH-12 with nothing stored a ${scheme} system gives a ${scheme} page`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await page.goto('/');
      if (scheme === 'dark') {
        await expectDark(page);
      } else {
        await expectLight(page);
      }
      expect(await stored(page)).toBeNull();
    });
  }

  test('TH-13 the dark class is set by the time the document has loaded', async ({ page }) => {
    await store(page, 'dark');
    await page.addInitScript(() => {
      document.addEventListener('DOMContentLoaded', () => {
        window.__darkAtContentLoaded = document.documentElement.classList.contains('dark');
      });
    });
    await page.goto('/');
    expect(await page.evaluate(() => window.__darkAtContentLoaded)).toBe(true);
  });
});
