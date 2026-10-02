import http from 'node:http';
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

function withTarget(response, requestUrl) {
  const { pathname, searchParams } = new URL(requestUrl);
  const typed = decodeURIComponent(pathname.split('/').pop());
  const [, bracketed, bracketedPort] = /^\[(.*)\](?::(\d+))?$/.exec(typed) || [];
  const [, name, namePort] = /^([^:]*)(?::(\d+))?$/.exec(typed) || [];
  const host = (bracketed ?? name ?? typed).toLowerCase();
  const port = Number(bracketedPort ?? namePort ?? (searchParams.get('bedrock') === 'true' ? 19132 : 25565));
  const isJson = response.status === 200 || (response.status === 404 && response.contentType === 'application/problem+json');
  if (!isJson) {
    return response;
  }
  return { ...response, body: JSON.stringify({ host, port, ...JSON.parse(response.body) }) };
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
    return route.fulfill(withTarget(await status(url), url.href));
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

  test('the address field text is readable in dark mode', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.goto('/project/mc-status');
    const foreground = await page.evaluate(() => getComputedStyle(document.body).color);
    await expect(page.locator('#mc-status-host')).toHaveCSS('color', foreground);
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
    await expect(page.locator('#mc-status-name')).toHaveText('play.example.net');
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
        motd: '<script>window.__xss=1</script>',
        version: '<b>1.21</b>',
        players: [{ name: '<i>mallory</i>' }],
      })
    );
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
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
    await page.route(`${API}/mcstatus/play.example.net*`, (route) => route.fulfill(withTarget(json(ONLINE), route.request().url())));
    await page.goto('/project/mc-status');
    const iconResponse = page.waitForResponse((res) => res.url().includes('/mcstatus/icon/'));
    await lookup(page, 'play.example.net');
    await iconResponse;
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-icon')).toHaveJSProperty('complete', true);
    await expect(page.locator('#mc-status-icon')).toBeHidden();
  });

  test('a failed icon on a later lookup hides the previous server icon', async ({ page }) => {
    await page.route(`${API}/mcstatus/icon/with-icon.example.net*`, (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1X1 })
    );
    await page.route(`${API}/mcstatus/icon/no-icon.example.net*`, (route) => route.fulfill({ status: 404 }));
    await page.route(`${API}/mcstatus/*.example.net*`, (route) => route.fulfill(withTarget(json(ONLINE), route.request().url())));
    await page.goto('/project/mc-status');

    await lookup(page, 'with-icon.example.net');
    await expect(page.locator('#mc-status-icon')).toBeVisible();

    const iconResponse = page.waitForResponse((res) => res.url().includes('/mcstatus/icon/no-icon'));
    await lookup(page, 'no-icon.example.net');
    await iconResponse;
    await expect(page.locator('#mc-status-icon')).toHaveJSProperty('complete', true);
    await expect(page.locator('#mc-status-icon')).toBeHidden();
  });

  test('a Bedrock lookup sends bedrock=true and no query, and loads the icon with bedrock=true', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await pickEdition(page, 'bedrock');
    await lookup(page, 'bedrock.example.net');

    await expect(page.locator('#mc-status-type')).toHaveText('Bedrock');
    expect(requests.status[0].searchParams.get('bedrock')).toBe('true');
    expect(requests.status[0].searchParams.has('query')).toBe(false);
    await expect(page.locator('#mc-status-icon')).toBeVisible();
    expect(requests.icon).toHaveLength(1);
    expect(requests.icon[0].pathname).toBe(`${API_PATH}/mcstatus/icon/bedrock.example.net`);
    expect(requests.icon[0].searchParams.get('bedrock')).toBe('true');
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

  test('the highest valid query port is sent', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await openAdvanced(page);
    await page.locator('#mc-status-query-port').fill('65535');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(requests.status[0].searchParams.get('query_port')).toBe('65535');
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

  for (const bad of ['0', '99999', '65536', '-5', '1.5', '1e3', '+80', '80a', ' 80', '80 ']) {
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
  test('a host in the path runs the lookup on load and fills the form', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-host')).toHaveValue('play.example.net');
    expect(requests.status).toHaveLength(1);
  });

  test('a host with a port is decoded from the path and kept readable in the shared URL', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/play.example.net%3A25570');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-host')).toHaveValue('play.example.net:25570');
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/play.example.net:25570');
  });

  test('an old ?host= link is ignored', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status?host=play.example.net');
    await expect(page.locator('#mc-status-host')).toHaveValue('');
    await expect(page.locator('#mc-status-result')).toBeHidden();
    expect(requests.status).toHaveLength(0);
  });

  test('the shared URL uses the canonical host and port from the API', async ({ page }) => {
    await mockMcStatus(page, () => json({ ...ONLINE, host: 'play.example.net', port: 80 }));
    await page.goto('/project/mc-status');
    await lookup(page, 'Play.Example.NET:00080');
    await expect(page).toHaveURL(/\/project\/mc-status\/play\.example\.net:80$/);
  });

  const typedHosts = {
    'a space': ['a b', 'a%20b'],
    'a non-ASCII name': ['bücher.de', 'b%C3%BCcher.de'],
    'a trailing dot': ['example.com.', 'example.com.'],
    'a path': ['a.com/b', 'a.com%2Fb'],
    'a scheme': ['http://a.com', 'http%3A%2F%2Fa.com'],
    'a query string': ['a.com?x=1', 'a.com%3Fx%3D1'],
    'upper case and a padded port': ['Play.Example.NET:00080', 'Play.Example.NET%3A00080'],
    'a bracketed IPv6 address': ['[2001:DB8::1]:25565', '%5B2001%3ADB8%3A%3A1%5D%3A25565'],
  };

  for (const [name, [typed, escaped]] of Object.entries(typedHosts)) {
    test(`${name} is sent to the API as typed`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto('/project/mc-status');
      await lookup(page, typed);
      await expect(page.locator('#mc-status-result')).toBeVisible();
      expect(requests.status[0].pathname).toBe(`${API_PATH}/mcstatus/${escaped}`);
    });
  }

  test('a host the API rejects shows its message and resets the URL to the bare checker', async ({ page }) => {
    const detail = 'The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port.';
    const requests = await mockMcStatus(page, (url) =>
      url.pathname.endsWith('/localhost')
        ? { status: 400, contentType: 'application/problem+json', body: JSON.stringify({ title: 'Bad Request', status: 400, detail }) }
        : json(ONLINE)
    );
    await page.goto('/project/mc-status/play.example.net?bedrock=true');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/play.example.net');

    await lookup(page, 'localhost');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    await expect(page.locator('#mc-status-error-detail')).toHaveText(detail);
    await expect(page.locator('#mc-status-result')).toBeHidden();
    expect(requests.status.at(-1).pathname).toBe(`${API_PATH}/mcstatus/localhost`);
    expect(new URL(page.url()).pathname).toBe('/project/mc-status');
    expect(new URL(page.url()).search).toBe('');
  });

  test('a rejected host in the path is cleared from the address bar and left in the form', async ({ page }) => {
    await mockMcStatus(page, () => ({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'bad host' }) }));
    await page.goto('/project/mc-status/bad%20host?query=false');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    await expect(page.locator('#mc-status-host')).toHaveValue('bad host');
    expect(new URL(page.url()).pathname).toBe('/project/mc-status');
    expect(new URL(page.url()).search).toBe('');
  });

  for (const [name, response] of Object.entries({
    '429': { status: 429, contentType: 'application/problem+json', body: '{}' },
    '500': { status: 500, contentType: 'application/problem+json', body: '{}' },
    'a 404 that is not a problem': { status: 404, contentType: 'text/html', body: 'Not Found' },
  })) {
    test(`a failed lookup (${name}) does not rewrite the URL`, async ({ page }) => {
      await mockMcStatus(page, () => response);
      await page.goto('/project/mc-status');
      await lookup(page, 'play.example.net');
      await expect(page.locator('#mc-status-error')).toBeVisible();
      expect(new URL(page.url()).pathname).toBe('/project/mc-status');
    });
  }

  test('an offline server puts its canonical host in the shared URL', async ({ page }) => {
    await mockMcStatus(page, () => ({
      status: 404,
      contentType: 'application/problem+json',
      body: JSON.stringify({ title: 'Not Found', status: 404, host: '2001:db8::1', port: 25566 }),
    }));
    await page.goto('/project/mc-status');
    await lookup(page, '[2001:DB8:0:0:0:0:0:1]:25566');
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/%5B2001:db8::1%5D:25566');
  });

  test('an offline problem without a host leaves the URL alone', async ({ page }) => {
    await page.route(`${API}/mcstatus/**`, (route) =>
      route.fulfill({ status: 404, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'down' }) })
    );
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    expect(new URL(page.url()).pathname).toBe('/project/mc-status');
  });

  const sharedPaths = {
    'a Java server on the default port': { edition: 'java', host: 'a.com', port: 25565, path: 'a.com', search: '' },
    'a Java server on another port': { edition: 'java', host: 'a.com', port: 25566, path: 'a.com:25566', search: '' },
    'a Java server on the Bedrock default': { edition: 'java', host: 'a.com', port: 19132, path: 'a.com:19132', search: '' },
    'a Bedrock server on the default port': { edition: 'bedrock', host: 'a.com', port: 19132, path: 'a.com', search: '?bedrock=true' },
    'a Bedrock server on another port': { edition: 'bedrock', host: 'a.com', port: 19133, path: 'a.com:19133', search: '?bedrock=true' },
    'a Bedrock server on the Java default': { edition: 'bedrock', host: 'a.com', port: 25565, path: 'a.com:25565', search: '?bedrock=true' },
    'an IPv6 server on the default port': { edition: 'java', host: '2001:db8::1', port: 25565, path: '%5B2001:db8::1%5D', search: '' },
    'an IPv6 server on another port': { edition: 'java', host: '2001:db8::1', port: 25566, path: '%5B2001:db8::1%5D:25566', search: '' },
    'a Bedrock IPv6 server on the default port': { edition: 'bedrock', host: '2001:db8::1', port: 19132, path: '%5B2001:db8::1%5D', search: '?bedrock=true' },
  };

  for (const [name, { edition, host, port, path, search }] of Object.entries(sharedPaths)) {
    test(`${name} gets ${path}${search} as its shared URL and name`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json({ ...ONLINE, host, port }));
      await page.goto('/project/mc-status');
      await pickEdition(page, edition);
      await lookup(page, 'typed.example.net');
      await expect(page.locator('#mc-status-result')).toBeVisible();
      expect(new URL(page.url()).pathname).toBe(`/project/mc-status/${path}`);
      expect(new URL(page.url()).search).toBe(search);
      await expect(page.locator('#mc-status-name')).toHaveText(decodeURIComponent(path));
      expect(requests.icon[0].pathname).toBe(`${API_PATH}/mcstatus/icon/${encodeURIComponent(decodeURIComponent(path))}`);
    });
  }

  test('an IPv6 host in the path fills the form and is looked up as typed', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/%5B2001:db8::1%5D:25566');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-host')).toHaveValue('[2001:db8::1]:25566');
    expect(requests.status[0].pathname).toBe(`${API_PATH}/mcstatus/%5B2001%3Adb8%3A%3A1%5D%3A25566`);
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/%5B2001:db8::1%5D:25566');
  });

  test('an IPv6 host with unescaped brackets in the path fills the form', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/[2001:db8::1]');
    await expect(page.locator('#mc-status-host')).toHaveValue('[2001:db8::1]');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/%5B2001:db8::1%5D');
  });

  test('a lookup rewrites the URL so the result can be shared', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/play.example.net');
    expect(new URL(page.url()).search).toBe('');
  });

  test('Bedrock and query=false are kept in the shared URL', async ({ page }) => {
    await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status');
    await pickEdition(page, 'bedrock');
    await lookup(page, 'bedrock.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/bedrock.example.net');
    expect(new URL(page.url()).search).toBe('?bedrock=true');

    await pickEdition(page, 'java');
    await openAdvanced(page);
    await page.locator('#mc-status-query').uncheck();
    await page.locator('#mc-status-submit').click();
    await expect(page).toHaveURL(/\/project\/mc-status\/bedrock\.example\.net\?query=false$/);
  });

  test('?bedrock=true selects Bedrock and skips the query', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/bedrock.example.net?bedrock=true');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('input[name="mc-edition"][value="bedrock"]')).toBeChecked();
    expect(requests.status[0].searchParams.get('bedrock')).toBe('true');
    expect(requests.status[0].searchParams.has('query')).toBe(false);
  });

  test('?query=false leaves the query box unticked', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/play.example.net?query=false');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query')).not.toBeChecked();
    expect(requests.status[0].searchParams.has('query')).toBe(false);
  });

  test('a valid ?query_port= fills the field, opens Advanced and is sent', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/play.example.net?query_port=25575');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query-port')).toHaveValue('25575');
    await expect(page.locator('#mc-status-advanced')).toHaveAttribute('open', '');
    expect(requests.status[0].searchParams.get('query_port')).toBe('25575');
  });

  test('an invalid ?query_port= is dropped from the field, the request and the URL', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto('/project/mc-status/play.example.net?query_port=0');
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-query-port')).toHaveValue('');
    await expect(page.locator('#mc-status-advanced')).not.toHaveAttribute('open', '');
    expect(requests.status[0].searchParams.has('query_port')).toBe(false);
    expect(new URL(page.url()).pathname).toBe('/project/mc-status/play.example.net');
    expect(new URL(page.url()).search).toBe('');
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

  for (const contentType of ['text/html', 'application/json', 'application/problem+xml', 'application/vnd.api+json', 'application/problem+json2', 'text/x; a=application/problem+json']) {
    test(`a 404 served as ${contentType} is not reported as an unreachable server`, async ({ page }) => {
      await mockMcStatus(page, () => ({ status: 404, contentType, body: '<h1>Not Found</h1>' }));
      await page.goto('/project/mc-status');
      await lookup(page, 'play.example.net');
      await expect(page.locator('#mc-status-error-message')).toHaveText('Unexpected response (404)');
    });
  }

  for (const contentType of ['application/problem+json; charset=utf-8', 'application/problem+json ; charset=utf-8', 'Application/Problem+JSON']) {
    test(`a 404 served as ${contentType} is reported as an unreachable server`, async ({ page }) => {
      await mockMcStatus(page, () => ({ status: 404, contentType, body: JSON.stringify({ detail: 'down' }) }));
      await page.goto('/project/mc-status');
      await lookup(page, 'play.example.net');
      await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    });
  }

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
    // Without pausing, real time keeps advancing the fake clock and the 30 s timer can fire early.
    await page.clock.pauseAt(new Date(Date.now() + 1000));
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');

    await page.clock.runFor(29_999);
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');

    await page.clock.runFor(1);
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    await expect(page.locator('#mc-status-error-detail')).toHaveText('The lookup timed out.');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });
});

test.describe('mc status page - request sequencing', () => {
  test('a superseded lookup whose error body arrives late does not show its error', async ({ page }) => {
    await page.addInitScript(() => {
      const realFetch = window.fetch.bind(window);
      window.fetch = (input, init) => {
        if (!String(input).includes('/mcstatus/slow404')) {
          return realFetch(input, init);
        }
        const body = new ReadableStream({
          start(controller) {
            window.__finishSlow404 = () => {
              controller.enqueue(new TextEncoder().encode('{"detail":"down"}'));
              controller.close();
            };
          },
        });
        return Promise.resolve(
          new Response(body, { status: 404, headers: { 'content-type': 'application/problem+json' } })
        );
      };
    });
    await page.route(`${API}/mcstatus/hang.example.net*`, () => new Promise(() => {}));

    await page.goto('/project/mc-status');
    await lookup(page, 'slow404');
    await expect.poll(() => page.evaluate(() => typeof window.__finishSlow404)).toBe('function');
    await lookup(page, 'hang.example.net');

    await page.evaluate(() => window.__finishSlow404());
    await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await expect(page.locator('#mc-status-error')).toBeHidden();
  });

  test('a superseded lookup leaves the button and banner to the newest one', async ({ page }) => {
    let releaseNewest;
    const newestReleased = new Promise((resolve) => {
      releaseNewest = resolve;
    });
    await page.route(`${API}/mcstatus/icon/**`, (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1X1 })
    );
    await page.route(`${API}/mcstatus/old.example.net*`, () => new Promise(() => {}));
    await page.route(`${API}/mcstatus/new.example.net*`, async (route) => {
      await newestReleased;
      await route.fulfill(withTarget(json(ONLINE), route.request().url()));
    });

    await page.goto('/project/mc-status');
    await lookup(page, 'old.example.net');
    await lookup(page, 'new.example.net');
    await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await expect(page.locator('#mc-status-error')).toBeHidden();

    releaseNewest();
    await expect(page.locator('#mc-status-name')).toHaveText('new.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });

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
      await route.fulfill(withTarget(json(ONLINE), route.request().url())).catch(() => {});
      slowServed();
    });
    await page.route(`${API}/mcstatus/fast.example.net*`, (route) => route.fulfill(withTarget(json(ONLINE), route.request().url())));
    await page.route(`${API}/mcstatus/last.example.net*`, (route) => route.fulfill(withTarget(json(ONLINE), route.request().url())));

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
    await expect(page.locator('#mc-status-name')).toHaveText('fast.example.net');

    releaseSlow();
    await slowDone;
    await lookup(page, 'last.example.net');
    await expect(page.locator('#mc-status-name')).toHaveText('last.example.net');
    expect(await page.evaluate(() => window.__names)).not.toContain('slow.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });
});

test.describe('mc status page - server-rendered route', () => {
  const rejectedByTheAPI = {
    'an illegal character': 'a_b$c',
    'a space': 'bad host',
    'a single-label name': 'localhost',
    'port 0': 'a.com:0',
    'port 65536': 'a.com:65536',
    'a bracketed IPv4 address': '[1.2.3.4]',
    'an IPv6 address with a zone': '[fe80::1%eth0]',
  };

  const html = async (request, path) => {
    const res = await request.get(path);
    expect(res.status()).toBe(200);
    return { res, body: await res.text() };
  };

  const expectBare = (res, body) => {
    expect(res.headers()['cache-control']).toBe('no-store');
    expect(body).toContain('id="mc-status-host"');
    expect(body).not.toContain('og:title');
  };

  test('an online server gets the preview tags and the checker form', async ({ request }) => {
    const { res, body } = await html(request, '/project/mc-status/Online.Example.NET');
    expect(res.headers()['cache-control']).toBe('public, max-age=60');
    expect(body).toContain('<title>online.example.net</title>');
    expect(body).toContain('property="og:title" content="online.example.net"');
    expect(body).toContain(`property="og:image" content="${process.env.NN_API_URL}/api/v1/mcstatus/icon/online.example.net"`);
    expect(body).toContain('Players: 3/20');
    expect(body).toContain('id="mc-status-host"');
  });

  test('the default port is left out of the title and canonical URL, another port is kept', async ({ request }) => {
    const site = process.env.BASE_URL || 'http://localhost:8099';
    const withDefault = await html(request, '/project/mc-status/online.example.net:25565');
    expect(withDefault.body).toContain('<title>online.example.net</title>');
    expect(withDefault.body).toContain(`rel="canonical" href="${site}/project/mc-status/online.example.net"`);

    const other = await html(request, '/project/mc-status/online.example.net:25566');
    expect(other.body).toContain('<title>online.example.net:25566</title>');
    expect(other.body).toContain(`rel="canonical" href="${site}/project/mc-status/online.example.net:25566"`);
  });

  test('an offline server gets the offline preview and the checker form', async ({ request }) => {
    const { res, body } = await html(request, '/project/mc-status/offline.example.net');
    expect(res.headers()['cache-control']).toBe('public, max-age=30');
    expect(body).toContain('<title>offline.example.net</title>');
    expect(body).toContain('Server offline or unreachable');
    expect(body).toContain('id="mc-status-host"');
  });

  test('a Bedrock preview asks for the Bedrock icon', async ({ request }) => {
    const { body } = await html(request, '/project/mc-status/online.example.net?bedrock=true');
    expect(body).toContain('<title>online.example.net</title>');
    expect(body).toContain(`property="og:image" content="${process.env.NN_API_URL}/api/v1/mcstatus/icon/online.example.net?bedrock=true"`);
  });

  test('an IPv6 server gets the preview tags with the bracketed canonical host', async ({ request }) => {
    for (const path of ['%5B2001:DB8::1%5D', '[2001:db8:0:0:0:0:0:1]', '2001:db8::1', '%5B2001:db8::1%5D:25565']) {
      const { body } = await html(request, `/project/mc-status/${path}`);
      expect(body).toContain('<title>[2001:db8::1]</title>');
      expect(body).toContain('property="og:title" content="[2001:db8::1]"');
      expect(body).toContain('property="og:image:alt" content="[2001:db8::1] server icon"');
      expect(body).toContain(`property="og:image" content="${process.env.NN_API_URL}/api/v1/mcstatus/icon/%5B2001:db8::1%5D"`);
      expect(body).toContain('/project/mc-status/%5B2001:db8::1%5D"');
      expect(body).toContain('Players: 3/20');
    }
  });

  for (const [name, host] of Object.entries(rejectedByTheAPI)) {
    test(`${name} is rejected by the API and gets the bare checker, not cached`, async ({ request }) => {
      const { res, body } = await html(request, `/project/mc-status/${encodeURIComponent(host)}`);
      expectBare(res, body);
    });
  }

  for (const state of ['online', 'offline']) {
    for (const missing of ['host', 'port']) {
      test(`an ${state} answer without a ${missing} gets the bare checker, not cached`, async ({ request }) => {
        const { res, body } = await html(request, `/project/mc-status/${state}-no-${missing}.example.net`);
        expectBare(res, body);
      });
    }
  }

  const apiRequests = async (request) => (await request.get(`${process.env.NN_API_URL}/__requests`)).json();
  const longHost = (last) => `${`${'a'.repeat(60)}.`.repeat(4)}${last}`;

  test('a host of 260 characters is looked up', async ({ request }) => {
    const host = longHost('b'.repeat(16));
    expect(host).toHaveLength(260);
    await html(request, `/project/mc-status/${host}`);
    expect((await apiRequests(request)).some((url) => url.includes(host))).toBe(true);
  });

  test('a host over 260 characters gets the bare checker without a lookup', async ({ request }) => {
    const host = longHost('c'.repeat(17));
    expect(host).toHaveLength(261);
    const { res, body } = await html(request, `/project/mc-status/${host}`);
    expectBare(res, body);
    expect((await apiRequests(request)).some((url) => url.includes(host))).toBe(false);
  });

  // Playwright's request client collapses %2e segments itself, so the raw path goes out over node:http.
  const rawGet = (path) =>
    new Promise((resolve, reject) => {
      const base = new URL(process.env.BASE_URL || 'http://localhost:8099');
      http
        .get({ host: base.hostname, port: base.port, path }, (res) => {
          let body = '';
          res.setEncoding('utf8');
          res.on('data', (chunk) => (body += chunk));
          res.on('end', () => resolve({ res: { status: () => res.statusCode, headers: () => res.headers }, body }));
        })
        .on('error', reject);
    });

  for (const segment of ['%2e', '%2e%2e']) {
    test(`the dot segment ${segment} gets the bare checker without a lookup`, async ({ request }) => {
      const { res, body } = await rawGet(`/project/mc-status/${segment}`);
      expect(res.status()).toBe(200);
      expect(res.headers()['cache-control']).toBe('no-store');
      expect(body).toContain('id="mc-status-host"');
      expect(body).not.toContain('og:title');
      expect((await apiRequests(request)).filter((url) => url.startsWith('/api/v1/mcstatus/.'))).toEqual([]);
    });
  }
});
