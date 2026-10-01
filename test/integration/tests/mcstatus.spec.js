import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;
const API_PATH = new URL(API).pathname;

const PNG_1X1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
  'base64'
);

const ONLINE = {
  name: 'Hypixel Network',
  motd: '§aHello\\n§c§lWorld',
  version: 'Paper 1.21',
  num_players: 3,
  max_players: 20,
  players: [{ name: 'Alex' }, { name: 'Steve' }],
};

function json(body, status = 200) {
  return { status, contentType: 'application/json', body: JSON.stringify(body) };
}

async function mockMcStatus(page, status) {
  const requests = { status: [], icon: [] };
  await page.route(`${API}/mcstatus/**`, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.startsWith(`${API_PATH}/mcstatus/icon/`)) {
      requests.icon.push(url);
      return route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1X1 });
    }
    requests.status.push(url);
    return route.fulfill(await status(url));
  });
  return requests;
}

async function lookup(page, host) {
  await page.locator('#mc-status-host').fill(host);
  await page.locator('#mc-status-submit').click();
}

async function pickEdition(page, edition) {
  await page.locator(`label:has(input[name="mc-edition"][value="${edition}"])`).click();
}

async function openAdvanced(page) {
  await page.locator('#mc-status-advanced summary').click();
}

test.describe('mc status page - form', () => {
  test('renders with Java selected and the advanced options collapsed', async ({ page }) => {
    await page.goto('/project/mc-status');
    await expect(page.locator('#mc-status-host')).toBeVisible();
    await expect(page.locator('input[name="mc-edition"][value="java"]')).toBeChecked();
    await expect(page.locator('#mc-status-advanced')).not.toHaveAttribute('open', '');
    await expect(page.locator('#mc-status-result')).toBeHidden();
    await expect(page.locator('#mc-status-error')).toBeHidden();
  });

  test('the query box is on by default and disabled for Bedrock, restoring the choice after', async ({ page }) => {
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    const query = page.locator('#mc-status-query');
    await expect(query).toBeChecked();

    await query.uncheck();
    await pickEdition(page, 'bedrock');
    await expect(query).toBeDisabled();
    await expect(query).not.toBeChecked();
    await expect(page.locator('#mc-status-query-port')).toBeDisabled();

    await pickEdition(page, 'java');
    await expect(query).toBeEnabled();
    await expect(query).not.toBeChecked();
  });

  test('the query box stays ticked across a Bedrock round trip when it was never touched', async ({ page }) => {
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    await pickEdition(page, 'bedrock');
    await pickEdition(page, 'java');
    await expect(page.locator('#mc-status-query')).toBeChecked();
  });

  test('the query port is only enabled while the query box is ticked', async ({ page }) => {
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    const port = page.locator('#mc-status-query-port');
    await expect(port).toBeEnabled();
    await page.locator('#mc-status-query').uncheck();
    await expect(port).toBeDisabled();
    await page.locator('#mc-status-query').check();
    await expect(port).toBeEnabled();
  });

  test('a dot-segment address shows an error without calling the API', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await page.locator('#mc-status-host').fill('..');
    await page.locator('#mc-status-submit').click();
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    expect(requests.status).toHaveLength(0);
  });
});

test.describe('mc status page - lookups', () => {
  test('a Java lookup requests the query and renders the result card', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');

    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-name')).toHaveText('Hypixel Network');
    await expect(page.locator('#mc-status-pill')).toHaveText('Online');
    await expect(page.locator('#mc-status-type')).toHaveText('Java');
    await expect(page.locator('#mc-status-version')).toHaveText('Paper 1.21');
    await expect(page.locator('#mc-status-players-count')).toHaveText('3 / 20');
    await expect(page.locator('#mc-status-players-bar')).toHaveAttribute('style', /width:\s*15%/);
    await expect(page.locator('[role="progressbar"]')).toHaveAttribute('aria-valuenow', '3');
    await expect(page.locator('[role="progressbar"]')).toHaveAttribute('aria-valuemax', '20');

    expect(requests.status).toHaveLength(1);
    expect(requests.status[0].pathname).toBe(`${API_PATH}/mcstatus/play.example.net`);
    expect(requests.status[0].searchParams.get('query')).toBe('true');
    expect(requests.status[0].searchParams.has('bedrock')).toBe(false);
  });

  test('colour codes are stripped and the literal \\n becomes a line break', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-motd')).toHaveText('Hello\nWorld');
  });

  test('hex colour runs are stripped from the MOTD', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, motd: '§x§f§f§0§0§0§0Red text' }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-motd')).toHaveText('Red text');
  });

  test('server-supplied text is rendered as text, never as HTML', async ({ page }) => {
    await mockMcStatus(page, () =>
      json({
        ...ONLINE,
        name: '<img src=x onerror=window.__xss=1>',
        motd: '<script>window.__xss=1</script>',
        version: '<b>1.21</b>',
        players: [{ name: '<i>mallory</i>' }],
      })
    );
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-name')).toHaveText('<img src=x onerror=window.__xss=1>');
    await expect(page.locator('#mc-status-motd')).toHaveText('<script>window.__xss=1</script>');
    await expect(page.locator('#mc-status-version')).toHaveText('<b>1.21</b>');
    await expect(page.locator('#mc-status-players li').first()).toHaveText('<i>mallory</i>');
    await expect(page.locator('#mc-status-result img:not(#mc-status-icon)')).toHaveCount(0);
    expect(await page.evaluate(() => window.__xss)).toBeUndefined();
  });

  test('lists the sampled players and counts the ones it cannot show', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, num_players: 10 }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-players li')).toHaveText(['Alex', 'Steve', 'and 8 more']);
    await expect(page.locator('#mc-status-players-unavailable')).toBeHidden();
  });

  test('says the player list is unavailable when players are online but none are listed', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, players: [] }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-players-unavailable')).toBeVisible();
  });

  test('an empty server shows no unavailable notice and an empty bar', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, num_players: 0, players: [] }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-players-count')).toHaveText('0 / 20');
    await expect(page.locator('#mc-status-players-bar')).toHaveAttribute('style', /width:\s*0%/);
    await expect(page.locator('#mc-status-players-unavailable')).toBeHidden();
  });

  test('an over-full server clamps the bar and progress value to the maximum', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, num_players: 30, max_players: 20 }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-players-count')).toHaveText('30 / 20');
    await expect(page.locator('#mc-status-players-bar')).toHaveAttribute('style', /width:\s*100%/);
    await expect(page.locator('[role="progressbar"]')).toHaveAttribute('aria-valuenow', '20');
  });

  test('falls back to "Unknown version" when the API has none', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, version: '' }));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-version')).toHaveText('Unknown version');
  });

  test('the Java icon comes from the icon endpoint and shows once it loads', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net:25570');
    await expect(page.locator('#mc-status-icon')).toBeVisible();
    expect(requests.icon).toHaveLength(1);
    expect(requests.icon[0].pathname).toBe(`${API_PATH}/mcstatus/icon/play.example.net%3A25570`);
  });

  test('the icon stays hidden when the icon request fails', async ({ page }) => {
    await page.route(`${API}/mcstatus/icon/**`, (route) => route.fulfill({ status: 404 }));
    await page.route(`${API}/mcstatus/play.example.net*`, (route) => route.fulfill(json(ONLINE)));
    await page.goto('/project/mc-status');
    const iconResponse = page.waitForResponse((res) => res.url().includes('/mcstatus/icon/'));
    await lookup(page, 'play.example.net');
    await iconResponse;
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-icon')).toHaveJSProperty('complete', true);
    await expect(page.locator('#mc-status-icon')).toBeHidden();
  });

  test('a Bedrock lookup sends bedrock=true, no query, and no icon request', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await pickEdition(page, 'bedrock');
    await lookup(page, 'bedrock.example.net');

    await expect(page.locator('#mc-status-type')).toHaveText('Bedrock');
    expect(requests.status[0].searchParams.get('bedrock')).toBe('true');
    expect(requests.status[0].searchParams.has('query')).toBe(false);
    await expect(page.locator('#mc-status-icon')).not.toHaveAttribute('src', /.+/);
    expect(requests.icon).toHaveLength(0);
  });

  test('turning the query off sends no query parameter', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    await page.locator('#mc-status-query').uncheck();
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(requests.status[0].searchParams.has('query')).toBe(false);
  });
});

test.describe('mc status page - query port', () => {
  test('a valid query port is sent as query_port', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    await page.locator('#mc-status-query-port').fill('25575');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(requests.status[0].searchParams.get('query')).toBe('true');
    expect(requests.status[0].searchParams.get('query_port')).toBe('25575');
    expect(new URL(page.url()).searchParams.get('query_port')).toBe('25575');
  });

  test('a blank query port sends no query_port', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(requests.status[0].searchParams.has('query_port')).toBe(false);
  });

  test('the query port is not sent when the query is off', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    await page.locator('#mc-status-query-port').fill('25575');
    await page.locator('#mc-status-query').uncheck();
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(requests.status[0].searchParams.has('query_port')).toBe(false);
    expect(new URL(page.url()).searchParams.has('query_port')).toBe(false);
  });

  for (const bad of ['0', '99999', '-5', '1.5']) {
    test(`an invalid query port (${bad}) opens Advanced and blocks the lookup`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto('/project/mc-status');
      await openAdvanced(page);
      await page.locator('#mc-status-query-port').fill(bad);
      await page.locator('#mc-status-advanced summary').click();
      await expect(page.locator('#mc-status-advanced')).not.toHaveAttribute('open', '');

      await lookup(page, 'play.example.net');
      await expect(page.locator('#mc-status-advanced')).toHaveAttribute('open', '');
      await expect(page.locator('#mc-status-query-port')).toBeFocused();
        expect(requests.status).toHaveLength(0);
    });
  }
});

test.describe('mc status page - share links', () => {
  test('?host= runs the lookup on load and fills the form', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-host')).toHaveValue('play.example.net');
    expect(requests.status).toHaveLength(1);
  });

  test('a lookup rewrites the URL so the result can be shared', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).search).toBe('?host=play.example.net');
  });

  test('Bedrock and query=false are kept in the shared URL', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await pickEdition(page, 'bedrock');
    await lookup(page, 'bedrock.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).search).toBe('?host=bedrock.example.net&bedrock=true');

    await pickEdition(page, 'java');
    await openAdvanced(page);
    await page.locator('#mc-status-query').uncheck();
    await page.locator('#mc-status-submit').click();
    await expect(page).toHaveURL(/\?host=bedrock\.example\.net&query=false$/);
  });

  test('?bedrock=true selects Bedrock and skips the query', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=bedrock.example.net&bedrock=true');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('input[name="mc-edition"][value="bedrock"]')).toBeChecked();
    expect(requests.status[0].searchParams.get('bedrock')).toBe('true');
    expect(requests.status[0].searchParams.has('query')).toBe(false);
  });

  test('?query=false leaves the query box unticked', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=play.example.net&query=false');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query')).not.toBeChecked();
    expect(requests.status[0].searchParams.has('query')).toBe(false);
  });

  test('a valid ?query_port= fills the field, opens Advanced and is sent', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=play.example.net&query_port=25575');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query-port')).toHaveValue('25575');
    await expect(page.locator('#mc-status-advanced')).toHaveAttribute('open', '');
    expect(requests.status[0].searchParams.get('query_port')).toBe('25575');
  });

  test('an invalid ?query_port= is dropped from the field, the request and the URL', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=play.example.net&query_port=0');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query-port')).toHaveValue('');
    await expect(page.locator('#mc-status-advanced')).not.toHaveAttribute('open', '');
    expect(requests.status[0].searchParams.has('query_port')).toBe(false);
    expect(new URL(page.url()).search).toBe('?host=play.example.net');
  });
});

test.describe('mc status page - errors', () => {
  const problem = (status, detail) => ({
    status,
    contentType: 'application/problem+json',
    body: JSON.stringify({ title: 'x', status, detail }),
  });

  test('404 (no status response) says the server could not be reached', async ({ page }) => {
    await mockMcStatus(page, () => problem(404, 'server did not respond'));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error')).toBeVisible();
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    await expect(page.locator('#mc-status-error-detail')).toHaveText('server did not respond');
    await expect(page.locator('#mc-status-result')).toBeHidden();
  });

  test('429 asks the user to try again in a minute', async ({ page }) => {
    await mockMcStatus(page, () => problem(429, 'rate limited'));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Too many lookups');
    await expect(page.locator('#mc-status-error-detail')).toHaveText('Please try again in a minute.');
  });

  test('500 reports a generic failure with the API detail', async ({ page }) => {
    await mockMcStatus(page, () => problem(500, 'boom'));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Something went wrong');
    await expect(page.locator('#mc-status-error-detail')).toHaveText('boom');
  });

  test('any other status is reported as unexpected, not guessed at', async ({ page }) => {
    await mockMcStatus(page, () => problem(502, 'bad gateway'));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Unexpected response (502)');
  });

  test('a network failure asks the user to check their connection', async ({ page }) => {
    await page.route(`${API}/mcstatus/**`, (route) => route.abort('failed'));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Something went wrong');
    await expect(page.locator('#mc-status-error-detail')).toHaveText('Check your connection and try again.');
  });

  test('an error replaces a previous result, and a later success replaces the error', async ({ page }) => {
    let fail = false;
    await mockMcStatus(page, () => (fail ? problem(404, 'down') : json(ONLINE)));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();

    fail = true;
    await page.locator('#mc-status-submit').click();
    await expect(page.locator('#mc-status-error')).toBeVisible();
    await expect(page.locator('#mc-status-result')).toBeHidden();

    fail = false;
    await page.locator('#mc-status-submit').click();
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-error')).toBeHidden();
  });
});

test.describe('mc status page - timeout', () => {
  test('a lookup that never answers times out with a message and frees the button', async ({ page }) => {
    await page.clock.install();
    await page.route(`${API}/mcstatus/**`, () => new Promise(() => {}));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');

    await page.clock.runFor(29_999);
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await expect(page.locator('#mc-status-error')).toBeHidden();

    await page.clock.runFor(1);
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    await expect(page.locator('#mc-status-error-detail')).toHaveText('The lookup timed out.');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });
});

test.describe('mc status page - request sequencing', () => {
  test('a slow earlier lookup cannot overwrite a newer one', async ({ page }) => {
    let releaseSlow;
    const slow = new Promise((resolve) => {
      releaseSlow = resolve;
    });
    let slowServed;
    const slowDone = new Promise((resolve) => {
      slowServed = resolve;
    });
    await page.route(`${API}/mcstatus/icon/**`, (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1X1 })
    );
    await page.route(`${API}/mcstatus/slow.example.net*`, async (route) => {
      await slow;
      await route.fulfill(json({ ...ONLINE, name: 'Slow' })).catch(() => {});
      slowServed();
    });
    await page.route(`${API}/mcstatus/fast.example.net*`, (route) => route.fulfill(json({ ...ONLINE, name: 'Fast' })));
    await page.route(`${API}/mcstatus/last.example.net*`, (route) => route.fulfill(json({ ...ONLINE, name: 'Last' })));

    await page.goto('/project/mc-status');
    await page.evaluate(() => {
      window.__names = [];
      new MutationObserver(() => window.__names.push(document.getElementById('mc-status-name').textContent)).observe(
        document.getElementById('mc-status-name'),
        { childList: true, characterData: true, subtree: true }
      );
    });

    await lookup(page, 'slow.example.net');
    await lookup(page, 'fast.example.net');
    await expect(page.locator('#mc-status-name')).toHaveText('Fast');

    releaseSlow();
    await slowDone;
    await lookup(page, 'last.example.net');
    await expect(page.locator('#mc-status-name')).toHaveText('Last');
    expect(await page.evaluate(() => window.__names)).not.toContain('Slow');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });
});

test.describe('mc status embed route', () => {
  const invalid = {
    'an illegal character': 'a_b$c',
    'a space': 'bad host',
    'port 0': 'a.com:0',
    'port 65536': 'a.com:65536',
    'a six digit port': 'a.com:123456',
    'a label over 63 characters': `${'a'.repeat(64)}.com`,
    'a name over 253 characters': `${`${'a'.repeat(60)}.`.repeat(5)}com`,
  };

  for (const [name, host] of Object.entries(invalid)) {
    test(`rejects ${name} with a 400 that is not cached`, async ({ request }) => {
      const res = await request.get(`/mcstatus/${encodeURIComponent(host)}`);
      expect(res.status()).toBe(400);
      expect(res.headers()['cache-control']).toBe('no-store');
    });
  }
});
