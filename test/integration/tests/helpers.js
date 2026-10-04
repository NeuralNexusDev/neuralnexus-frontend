import { randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

export { expect };

export const STUB = process.env.NN_API_URL;
export const APP = process.env.BASE_URL || 'http://localhost:8099';
const HTMX_CDN = 'https://cdn.neuralnexus.dev/htmx/htmx.v4.0.0.min.js';
const HTMX_FILE = fileURLToPath(new URL('../node_modules/htmx.org/dist/htmx.min.js', import.meta.url));

// Snowflake-sized IDs, above 2^53, matching the ones stub-admin.mjs seeds.
export const ID = {
  system: '3541025163146757610',
  owner: '3541025163146757620',
  bee: '3541025163146757630',
  pBee: '3541025163146757710',
  pRate: '3541025163146757730',
  pPets: '3541025163146757780',
  pMotd: '3541025163146757790',
  pStore: '3541025163146757800',
  alice: '3541025163146759010',
  bob: '3541025163146759020',
  anon: '3541025163146759030',
};

export const HOSTILE = '"><img src=x onerror="window.__xss=1">';

export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route(HTMX_CDN, (route) => route.fulfill({ contentType: 'application/javascript', headers: { 'access-control-allow-origin': '*' }, path: HTMX_FILE }));
    await use(page);
  },
});

/** Seeds a session on the stub API and signs the page in with it, since the app forwards that cookie to the API. */
export async function signIn(page, state = {}) {
  const session = randomUUID();
  const seeded = await page.request.post(`${STUB}/__admin/state`, { data: { session, state } });
  expect(seeded.ok()).toBe(true);
  await page.context().addCookies([{ name: 'session', value: session, url: APP }]);
  const calls = async () => (await page.request.get(`${STUB}/__admin/calls?session=${session}`)).json();
  const writes = async () =>
    (await calls())
      .filter((call) => call.method !== 'GET')
      .map(({ method, path, body }) => ({ method, path, body: body ?? null }));
  const api = (method, path, data) =>
    page.request.fetch(`${STUB}/api/v1${path}`, { method, data, headers: { Cookie: `session=${session}` } });
  return { session, calls, writes, api };
}

export async function expectNoInjection(page) {
  await expect(page.locator('img[src="x"]')).toHaveCount(0);
  expect(await page.evaluate(() => window.__xss)).toBeUndefined();
}

export const error = (page) => page.locator('#admin-error');
