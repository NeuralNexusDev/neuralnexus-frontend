import { randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

export { expect };

const STUB = process.env.NN_API_URL;
const APP = process.env.BASE_URL || 'http://localhost:8099';
const HTMX_CDN = 'https://s3.neuralnexus.dev/cdn/htmx/v4.0.0/htmx.min.js';
const HTMX_FILE = fileURLToPath(new URL('../node_modules/htmx.org/dist/htmx.min.js', import.meta.url));

// Snowflake-sized IDs, above 2^53, matching the ones stub-state.mjs seeds.
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

export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route(HTMX_CDN, (route) => route.fulfill({ contentType: 'application/javascript', headers: { 'access-control-allow-origin': '*' }, path: HTMX_FILE }));
    await use(page);
  },
});

/** Seeds a session on the stub API and signs the page in with it, since the app forwards that cookie to the API. */
export async function signIn(page, state = {}) {
  const session = randomUUID();
  const seeded = await page.request.post(`${STUB}/__state/state`, { data: { session, state } });
  expect(seeded.ok()).toBe(true);
  await page.context().addCookies([{ name: 'session', value: session, url: APP }]);
  const calls = async () => (await page.request.get(`${STUB}/__state/calls?session=${session}`)).json();
  const writes = async () =>
    (await calls())
      .filter((call) => call.method !== 'GET')
      .map(({ method, path, body }) => ({ method, path, body: body ?? null }));
  /** Holds the stub's answer to `key` ("METHOD /path") once `skip` calls have passed, so a test can act while the request is in flight. */
  const gate = async (key, { skip = 0 } = {}) => {
    const registered = await page.request.post(`${STUB}/__state/gate`, { data: { session, key, skip } });
    expect(registered.ok()).toBe(true);
    return {
      arrived: (count = 1) =>
        expect
          .poll(async () => (await (await page.request.get(`${STUB}/__state/gate?session=${session}&key=${encodeURIComponent(key)}`)).json()).arrived)
          .toBeGreaterThanOrEqual(count),
      /** Answers `outcome.index`, or every held call when it is absent. */
      release: async (outcome = {}) => {
        const released = await page.request.post(`${STUB}/__state/gate/release`, { data: { session, key, ...outcome } });
        expect(released.ok()).toBe(true);
      },
    };
  };
  return { session, calls, writes, gate };
}

export const error = (page) => page.locator('#page-error');
