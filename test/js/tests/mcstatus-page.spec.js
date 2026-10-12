import { test, expect } from '@playwright/test';

const API = `${process.env.NN_API_URL}/api/v1`;
const API_PATH = new URL(API).pathname;
const PAGE = '/project/mc-status';

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

const problem = (status, detail) => ({
  status,
  contentType: 'application/problem+json',
  body: JSON.stringify({ title: 'x', status, detail }),
});

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

const current = (page) => {
  const url = new URL(page.url());
  return { pathname: url.pathname, search: url.search };
};

test.describe('helpers', () => {
  test('MP-01 only . and .. are dot segments', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      ['.', true],
      ['..', true],
      ['...', false],
      ['.a', false],
      ['a.', false],
      ['..a', false],
      ['a..b', false],
      [' ..', false],
      ['', false],
    ];
    for (const [name, expected] of cases) {
      expect(await page.evaluate((value) => isDotSegment(value), name), JSON.stringify(name)).toBe(expected);
    }
  });

  test('MP-02 the MOTD loses colour codes and hex runs, turns a literal \\n into a line break and is trimmed', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      [ONLINE.motd, 'Hello\nWorld'],
      ['§x§f§f§0§0§0§0Red text', 'Red text'],
      ['one\\ntwo\\nthree', 'one\ntwo\nthree'],
      ['  padded  ', 'padded'],
      ['§Aupper', 'upper'],
      ['§😀x', 'x'],
      ['§', '§'],
      ['a§', 'a§'],
      ['', ''],
    ];
    for (const [motd, expected] of cases) {
      expect(await page.evaluate((value) => formatMcMotd(value), motd), JSON.stringify(motd)).toBe(expected);
    }
  });

  test('MP-03 a port is a whole number from 1 to 65535 written in plain digits', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      ['1', 1],
      ['25565', 25565],
      ['65535', 65535],
      ['0080', 80],
      ['0', null],
      ['65536', null],
      ['99999999999999999999', null],
      ['-5', null],
      ['1.5', null],
      ['1e3', null],
      ['+80', null],
      ['80a', null],
      [' 80', null],
      ['٣', null],
      ['', null],
    ];
    for (const [value, expected] of cases) {
      expect(await page.evaluate((text) => parseMcPort(text), value), JSON.stringify(value)).toBe(expected);
    }
  });

  test('MP-04 the display host brackets an IPv6 address and leaves out the default port of the edition', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      [{ host: 'a.com', port: 25565 }, false, 'a.com'],
      [{ host: 'a.com', port: 19132 }, false, 'a.com:19132'],
      [{ host: 'a.com', port: 19132 }, true, 'a.com'],
      [{ host: 'a.com', port: 25565 }, true, 'a.com:25565'],
      [{ host: 'a.com', port: 80 }, true, 'a.com:80'],
      [{ host: '2001:db8::1', port: 25565 }, false, '[2001:db8::1]'],
      [{ host: '2001:db8::1', port: 25566 }, false, '[2001:db8::1]:25566'],
      [{ host: '2001:db8::1', port: 19132 }, true, '[2001:db8::1]'],
    ];
    for (const [target, bedrock, expected] of cases) {
      expect(await page.evaluate(([t, b]) => mcDisplayHost(t, b), [target, bedrock]), JSON.stringify([target, bedrock])).toBe(expected);
    }
  });

  test('MP-05 the shared URL keeps the host readable, adds the search, and replaces the history entry', async ({ page }) => {
    await page.goto(PAGE);
    const before = await page.evaluate(() => history.length);
    const cases = [
      ['a.com', '', '/project/mc-status/a.com', ''],
      ['a.com:25570', '', '/project/mc-status/a.com:25570', ''],
      ['[2001:db8::1]:25566', '', '/project/mc-status/%5B2001:db8::1%5D:25566', ''],
      ['a b', 'bedrock=true', '/project/mc-status/a%20b', '?bedrock=true'],
      ['a.com/b', 'query=false', '/project/mc-status/a.com%2Fb', '?query=false'],
    ];
    for (const [host, search, pathname, expected] of cases) {
      await page.evaluate(([h, s]) => setMcStatusUrl(h, s), [host, search]);
      expect(current(page), host).toEqual({ pathname, search: expected });
    }
    expect(await page.evaluate(() => history.length)).toBe(before);
  });

  test('MP-06 a query port that is not a port is marked invalid with a message, and a blank or valid one is not', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      ['', ''],
      ['25575', ''],
      ['65535', ''],
      ['0', 'Enter a port from 1 to 65535'],
      ['65536', 'Enter a port from 1 to 65535'],
      [' 80', 'Enter a port from 1 to 65535'],
      ['80a', 'Enter a port from 1 to 65535'],
      ['', ''],
    ];
    for (const [value, message] of cases) {
      const shown = await page.evaluate((text) => {
        const input = document.getElementById('mc-status-query-port');
        input.value = text;
        validateMcQueryPort();
        return input.validationMessage;
      }, value);
      expect(shown, JSON.stringify(value)).toBe(message);
    }
  });

  test('MP-07 the query port in use is null when the field is off, blank or invalid, and the number when it is valid', async ({ page }) => {
    await page.goto(PAGE);
    const read = (value) =>
      page.evaluate((text) => {
        document.getElementById('mc-status-query-port').value = text;
        return mcStatusQueryPort();
      }, value);
    expect(await read('25575')).toBe(25575);
    expect(await read('')).toBeNull();
    expect(await read('0')).toBeNull();
    expect(await read('80a')).toBeNull();
    await page.evaluate(() => document.getElementById('mc-status-query').click());
    await expect(page.locator('#mc-status-query-port')).toBeDisabled();
    expect(await read('25575')).toBeNull();
  });
});

test.describe('result card', () => {
  test('MP-10 the card shows the counts, the bar, the aria values and the player chips for each kind of answer', async ({ page }) => {
    await page.goto(PAGE);
    const cases = [
      { name: 'a sample one short of the count', status: ONLINE, counts: '3 / 20', width: '15%', now: '3', max: '20', chips: ['Alex', 'Steve', 'and 1 more'], unavailable: false },
      { name: 'more online than listed', status: { ...ONLINE, num_players: 10 }, counts: '10 / 20', width: '50%', now: '10', max: '20', chips: ['Alex', 'Steve', 'and 8 more'], unavailable: false },
      { name: 'players online and none listed', status: { ...ONLINE, players: [] }, counts: '3 / 20', width: '15%', now: '3', max: '20', chips: [], unavailable: true },
      { name: 'an empty server', status: { ...ONLINE, num_players: 0, players: [] }, counts: '0 / 20', width: '0%', now: '0', max: '20', chips: [], unavailable: false },
      { name: 'over full', status: { ...ONLINE, num_players: 30, max_players: 20 }, counts: '30 / 20', width: '100%', now: '20', max: '20', chips: ['Alex', 'Steve', 'and 28 more'], unavailable: false },
      { name: 'no maximum', status: { ...ONLINE, num_players: 5, max_players: 0 }, counts: '5 / 0', width: '0%', now: '0', max: '0', chips: ['Alex', 'Steve', 'and 3 more'], unavailable: false },
      { name: 'an answer with no counts, version or MOTD', status: {}, counts: '0 / 0', width: '0%', now: '0', max: '0', chips: [], unavailable: false, version: 'Unknown version', motd: '' },
      { name: 'an empty version', status: { ...ONLINE, version: '' }, counts: '3 / 20', width: '15%', now: '3', max: '20', chips: ['Alex', 'Steve', 'and 1 more'], unavailable: false, version: 'Unknown version' },
    ];
    for (const { name, status, counts, width, now, max, chips, unavailable, version = status.version, motd = 'Hello\nWorld' } of cases) {
      await page.evaluate((s) => renderMcStatus(s, false, 'play.example.net'), status);
      await expect(page.locator('#mc-status-players-count'), name).toHaveText(counts);
      await expect(page.locator('#mc-status-players-bar'), name).toHaveAttribute('style', new RegExp(`width:\\s*${width}`));
      await expect(page.locator('[role="progressbar"]'), name).toHaveAttribute('aria-valuenow', now);
      await expect(page.locator('[role="progressbar"]'), name).toHaveAttribute('aria-valuemax', max);
      await expect(page.locator('#mc-status-players li'), name).toHaveText(chips);
      await expect(page.locator('#mc-status-players-unavailable'), name)[unavailable ? 'toBeVisible' : 'toBeHidden']();
      await expect(page.locator('#mc-status-version'), name).toHaveText(version);
      expect(await page.locator('#mc-status-motd').evaluate((element) => element.textContent), name).toBe(motd);
    }
  });

  test('MP-11 text from the server is shown as text and never as HTML', async ({ page }) => {
    await page.goto(PAGE);
    await page.evaluate(() =>
      renderMcStatus(
        {
          motd: '<script>window.__xss=1</script>',
          version: '<b>1.21</b>',
          num_players: 1,
          max_players: 2,
          players: [{ name: '<i>mallory</i>' }],
        },
        false,
        '<u>host</u>'
      )
    );
    await expect(page.locator('#mc-status-name')).toHaveText('<u>host</u>');
    await expect(page.locator('#mc-status-motd')).toHaveText('<script>window.__xss=1</script>');
    await expect(page.locator('#mc-status-version')).toHaveText('<b>1.21</b>');
    await expect(page.locator('#mc-status-players li').first()).toHaveText('<i>mallory</i>');
    await expect(page.locator('#mc-status-result img:not(#mc-status-icon), #mc-status-result u, #mc-status-result b, #mc-status-result i')).toHaveCount(0);
    expect(await page.evaluate(() => window.__xss)).toBeUndefined();
  });

  test('MP-12 the icon comes from the icon endpoint with the edition and shows once it has loaded', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    await page.goto(PAGE);
    await page.evaluate(() => renderMcStatus({}, false, 'play.example.net:25570'));
    await expect(page.locator('#mc-status-icon')).toBeVisible();
    await page.evaluate(() => renderMcStatus({}, true, 'bedrock.example.net'));
    await expect(page.locator('#mc-status-icon')).toBeVisible();
    expect(requests.icon.map((url) => url.pathname + url.search)).toEqual([
      `${API_PATH}/mcstatus/icon/play.example.net%3A25570`,
      `${API_PATH}/mcstatus/icon/bedrock.example.net?bedrock=true`,
    ]);
  });

  test('MP-12 the icon stays hidden when the icon request fails', async ({ page }) => {
    await page.route(`${API}/mcstatus/icon/**`, (route) => route.fulfill({ status: 404 }));
    await page.goto(PAGE);
    const iconResponse = page.waitForResponse((res) => res.url().includes('/mcstatus/icon/'));
    await page.evaluate(() => renderMcStatus({}, false, 'play.example.net'));
    await iconResponse;
    await expect(page.locator('#mc-status-result')).toBeVisible();
    await expect(page.locator('#mc-status-icon')).toHaveJSProperty('complete', true);
    await expect(page.locator('#mc-status-icon')).toBeHidden();
  });

  test('MP-12 a failed icon on a later render hides the icon of the server before it', async ({ page }) => {
    await page.route(`${API}/mcstatus/icon/with-icon.example.net*`, (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1X1 })
    );
    await page.route(`${API}/mcstatus/icon/no-icon.example.net*`, (route) => route.fulfill({ status: 404 }));
    await page.goto(PAGE);
    await page.evaluate(() => renderMcStatus({}, false, 'with-icon.example.net'));
    await expect(page.locator('#mc-status-icon')).toBeVisible();

    const iconResponse = page.waitForResponse((res) => res.url().includes('/mcstatus/icon/no-icon'));
    await page.evaluate(() => renderMcStatus({}, false, 'no-icon.example.net'));
    await iconResponse;
    await expect(page.locator('#mc-status-icon')).toHaveJSProperty('complete', true);
    await expect(page.locator('#mc-status-icon')).toBeHidden();
  });

  test('MP-13 a later render replaces the chips and the notice of the one before and takes the error away', async ({ page }) => {
    await page.goto(PAGE);
    await page.evaluate(() => showMcStatusError('Something went wrong', 'boom'));
    await page.evaluate((status) => renderMcStatus(status, false, 'a.example.net'), { ...ONLINE, num_players: 10 });
    await expect(page.locator('#mc-status-players li')).toHaveCount(3);
    await expect(page.locator('#mc-status-error')).toBeHidden();
    await page.evaluate((status) => renderMcStatus(status, true, 'b.example.net'), { ...ONLINE, num_players: 1, players: [] });
    await expect(page.locator('#mc-status-players li')).toHaveCount(0);
    await expect(page.locator('#mc-status-players-unavailable')).toBeVisible();
    await expect(page.locator('#mc-status-type')).toHaveText('Bedrock');
    await expect(page.locator('#mc-status-name')).toHaveText('b.example.net');
    await expect(page.locator('#mc-status-result')).toBeVisible();
  });
});

test.describe('error banner', () => {
  test('MP-14 the banner shows the message with its detail when there is one, and hides the result', async ({ page }) => {
    await page.goto(PAGE);
    await page.evaluate((status) => renderMcStatus(status, false, 'a.example.net'), ONLINE);
    await expect(page.locator('#mc-status-result')).toBeVisible();

    await page.evaluate(() => showMcStatusError('Too many lookups', 'Please try again in a minute.'));
    await expect(page.locator('#mc-status-error')).toBeVisible();
    await expect(page.locator('#mc-status-result')).toBeHidden();
    await expect(page.locator('#mc-status-error-message')).toHaveText('Too many lookups');
    await expect(page.locator('#mc-status-error-detail')).toBeVisible();
    await expect(page.locator('#mc-status-error-detail')).toHaveText('Please try again in a minute.');

    await page.evaluate(() => showMcStatusError('Enter a valid server address'));
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    await expect(page.locator('#mc-status-error-detail')).toBeHidden();
    await expect(page.locator('#mc-status-error-detail')).toHaveText('');
  });
});

test.describe('form', () => {
  test('MP-20 the query box is disabled for Bedrock and its choice comes back with Java', async ({ page }) => {
    await page.goto(PAGE);
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

  test('MP-20 the query box stays ticked across a Bedrock round trip when it was never touched', async ({ page }) => {
    await page.goto(PAGE);
    await openAdvanced(page);
    await pickEdition(page, 'bedrock');
    await pickEdition(page, 'java');
    await expect(page.locator('#mc-status-query')).toBeChecked();
  });

  test('MP-21 the query port is only enabled while the query box is ticked', async ({ page }) => {
    await page.goto(PAGE);
    await openAdvanced(page);
    const port = page.locator('#mc-status-query-port');
    await expect(port).toBeEnabled();
    await page.locator('#mc-status-query').uncheck();
    await expect(port).toBeDisabled();
    await page.locator('#mc-status-query').check();
    await expect(port).toBeEnabled();
  });

  test('MP-22 the address field text is readable in dark mode', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.goto(PAGE);
    const foreground = await page.evaluate(() => getComputedStyle(document.body).color);
    await expect(page.locator('#mc-status-host')).toHaveCSS('color', foreground);
  });
});

test.describe('lookup request', () => {
  const lookups = [
    {
      name: 'Java sends the query and shows the card',
      setup: async () => {},
      host: 'play.example.net',
      query: 'true',
      bedrock: null,
      queryPort: null,
      url: { pathname: `${PAGE}/play.example.net`, search: '' },
      card: true,
    },
    {
      name: 'Bedrock sends bedrock and no query, and loads the icon as Bedrock',
      setup: (page) => pickEdition(page, 'bedrock'),
      host: 'bedrock.example.net',
      query: null,
      bedrock: 'true',
      queryPort: null,
      url: { pathname: `${PAGE}/bedrock.example.net`, search: '?bedrock=true' },
      type: 'Bedrock',
    },
    {
      name: 'the query off sends no query',
      setup: async (page) => {
        await openAdvanced(page);
        await page.locator('#mc-status-query').uncheck();
      },
      host: 'play.example.net',
      query: null,
      bedrock: null,
      queryPort: null,
      url: { pathname: `${PAGE}/play.example.net`, search: '?query=false' },
    },
    {
      name: 'a query port is sent and kept in the URL',
      setup: async (page) => {
        await openAdvanced(page);
        await page.locator('#mc-status-query-port').fill('25575');
      },
      host: 'play.example.net',
      query: 'true',
      bedrock: null,
      queryPort: '25575',
      url: { pathname: `${PAGE}/play.example.net`, search: '?query_port=25575' },
    },
    {
      name: 'the highest query port is sent',
      setup: async (page) => {
        await openAdvanced(page);
        await page.locator('#mc-status-query-port').fill('65535');
      },
      host: 'play.example.net',
      query: 'true',
      bedrock: null,
      queryPort: '65535',
      url: { pathname: `${PAGE}/play.example.net`, search: '?query_port=65535' },
    },
    {
      name: 'a blank query port sends none',
      setup: async () => {},
      host: 'play.example.net',
      query: 'true',
      bedrock: null,
      queryPort: null,
      url: { pathname: `${PAGE}/play.example.net`, search: '' },
    },
    {
      name: 'a query port is not sent when the query is off',
      setup: async (page) => {
        await openAdvanced(page);
        await page.locator('#mc-status-query-port').fill('25575');
        await page.locator('#mc-status-query').uncheck();
      },
      host: 'play.example.net',
      query: null,
      bedrock: null,
      queryPort: null,
      url: { pathname: `${PAGE}/play.example.net`, search: '?query=false' },
    },
  ];

  for (const { name, setup, host, query, bedrock, queryPort, url, card, type } of lookups) {
    test(`MP-30 ${name}`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(PAGE);
      await setup(page);
      await lookup(page, host);
      await expect(page.locator('#mc-status-result')).toBeVisible();

      expect(requests.status).toHaveLength(1);
      expect(requests.status[0].pathname).toBe(`${API_PATH}/mcstatus/${host}`);
      expect(requests.status[0].searchParams.get('query')).toBe(query);
      expect(requests.status[0].searchParams.get('bedrock')).toBe(bedrock);
      expect(requests.status[0].searchParams.get('query_port')).toBe(queryPort);
      expect(current(page)).toEqual(url);
      if (type) {
        await expect(page.locator('#mc-status-type')).toHaveText(type);
        await expect(page.locator('#mc-status-icon')).toBeVisible();
        expect(requests.icon.map((icon) => icon.searchParams.get('bedrock'))).toEqual(['true']);
      }
      if (card) {
        await expect(page.locator('#mc-status-name')).toHaveText(host);
        await expect(page.locator('#mc-status-pill')).toHaveText('Online');
        await expect(page.locator('#mc-status-type')).toHaveText('Java');
        await expect(page.locator('#mc-status-version')).toHaveText('Paper 1.21');
        await expect(page.locator('#mc-status-players-count')).toHaveText('3 / 20');
        await expect(page.locator('#mc-status-players-bar')).toHaveAttribute('style', /width:\s*15%/);
        await expect(page.locator('#mc-status-icon')).toBeVisible();
        expect(requests.icon).toHaveLength(1);
      }
    });
  }

  for (const blank of ['..', '.', '   ']) {
    test(`MP-31 the address ${JSON.stringify(blank)} shows an error without calling the API`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(PAGE);
      await lookup(page, blank);
      await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
      await expect(page.locator('#mc-status-error-detail')).toBeHidden();
      await expect(page.locator('#mc-status-submit')).toHaveText('Check');
      expect(requests.status).toHaveLength(0);
    });
  }

  const typedHosts = {
    'a space': ['a b', 'a%20b'],
    'a non-ASCII name': ['bücher.de', 'b%C3%BCcher.de'],
    'a path': ['a.com/b', 'a.com%2Fb'],
    'a bracketed IPv6 address': ['[2001:DB8::1]:25565', '%5B2001%3ADB8%3A%3A1%5D%3A25565'],
  };

  for (const [name, [typed, escaped]] of Object.entries(typedHosts)) {
    test(`MP-32 ${name} is sent to the API as typed`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(PAGE);
      await lookup(page, typed);
      await expect(page.locator('#mc-status-result')).toBeVisible();
      expect(requests.status[0].pathname).toBe(`${API_PATH}/mcstatus/${escaped}`);
    });
  }

  const sharedPaths = {
    'a Java server on the default port': { edition: 'java', host: 'a.com', port: 25565, path: 'a.com', search: '' },
    'a Java server on the Bedrock default': { edition: 'java', host: 'a.com', port: 19132, path: 'a.com:19132', search: '' },
    'a Bedrock server on the default port': { edition: 'bedrock', host: 'a.com', port: 19132, path: 'a.com', search: '?bedrock=true' },
    'a Bedrock server on the Java default': { edition: 'bedrock', host: 'a.com', port: 25565, path: 'a.com:25565', search: '?bedrock=true' },
    'an IPv6 server on the default port': { edition: 'java', host: '2001:db8::1', port: 25565, path: '%5B2001:db8::1%5D', search: '' },
    'an IPv6 server on another port': { edition: 'java', host: '2001:db8::1', port: 25566, path: '%5B2001:db8::1%5D:25566', search: '' },
  };

  for (const [name, { edition, host, port, path, search }] of Object.entries(sharedPaths)) {
    test(`MP-33 ${name} gets ${path}${search} as its shared URL and name`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json({ ...ONLINE, host, port }));
      await page.goto(PAGE);
      await pickEdition(page, edition);
      await lookup(page, 'typed.example.net');
      await expect(page.locator('#mc-status-result')).toBeVisible();
      expect(current(page)).toEqual({ pathname: `${PAGE}/${path}`, search });
      await expect(page.locator('#mc-status-name')).toHaveText(decodeURIComponent(path));
      expect(requests.icon[0].pathname).toBe(`${API_PATH}/mcstatus/icon/${encodeURIComponent(decodeURIComponent(path))}`);
    });
  }

  for (const bad of ['0', '65536', '-5', '1.5', '1e3', '+80', '80a', ' 80']) {
    test(`MP-34 an invalid query port (${bad}) opens Advanced and blocks the lookup`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(PAGE);
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

test.describe('lookup answers', () => {
  const answers = [
    { name: '404 from the API says the server could not be reached', response: () => problem(404, 'server did not respond'), message: "Couldn't reach that server", detail: 'server did not respond' },
    { name: '404 that is not a problem document is not reported as an unreachable server', response: () => ({ status: 404, contentType: 'text/html', body: '<h1>Not Found</h1>' }), message: 'Unexpected response (404)', detail: null },
    { name: '429 asks the user to try again in a minute', response: () => problem(429, 'rate limited'), message: 'Too many lookups', detail: 'Please try again in a minute.' },
    { name: '500 reports a generic failure with the API detail', response: () => problem(500, 'boom'), message: 'Something went wrong', detail: 'boom' },
    { name: 'any other status is reported as unexpected', response: () => problem(502, 'bad gateway'), message: 'Unexpected response (502)', detail: 'bad gateway' },
  ];

  for (const { name, response, message, detail } of answers) {
    test(`MP-35 ${name}`, async ({ page }) => {
      await mockMcStatus(page, response);
      await page.goto(PAGE);
      await lookup(page, 'play.example.net');
      await expect(page.locator('#mc-status-error')).toBeVisible();
      await expect(page.locator('#mc-status-error-message')).toHaveText(message);
      if (detail !== null) {
        await expect(page.locator('#mc-status-error-detail')).toHaveText(detail);
      }
      await expect(page.locator('#mc-status-result')).toBeHidden();
    });
  }

  test('MP-35 a network failure asks the user to check their connection', async ({ page }) => {
    await page.route(`${API}/mcstatus/**`, (route) => route.abort('failed'));
    await page.goto(PAGE);
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Something went wrong');
    await expect(page.locator('#mc-status-error-detail')).toHaveText('Check your connection and try again.');
    await expect(page.locator('#mc-status-result')).toBeHidden();
  });

  test('MP-36 a host the API rejects resets the URL to the bare checker and shows its message', async ({ page }) => {
    const detail = 'The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port.';
    const requests = await mockMcStatus(page, (url) =>
      url.pathname.endsWith('/localhost')
        ? { status: 400, contentType: 'application/problem+json', body: JSON.stringify({ title: 'Bad Request', status: 400, detail }) }
        : json(ONLINE)
    );
    await page.goto(`${PAGE}/play.example.net?bedrock=true`);
    await expect(page.locator('#mc-status-result')).toBeVisible();
    expect(current(page).pathname).toBe(`${PAGE}/play.example.net`);

    await lookup(page, 'localhost');
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    await expect(page.locator('#mc-status-error-detail')).toHaveText(detail);
    await expect(page.locator('#mc-status-result')).toBeHidden();
    expect(requests.status.at(-1).pathname).toBe(`${API_PATH}/mcstatus/localhost`);
    expect(current(page)).toEqual({ pathname: PAGE, search: '' });
  });

  test('MP-36 a rejected host in the path is cleared from the address bar and left in the form', async ({ page }) => {
    await mockMcStatus(page, () => ({ status: 400, contentType: 'application/problem+json', body: JSON.stringify({ detail: 'bad host' }) }));
    await page.goto(`${PAGE}/bad%20host?query=false`);
    await expect(page.locator('#mc-status-error-message')).toHaveText('Enter a valid server address');
    await expect(page.locator('#mc-status-host')).toHaveValue('bad host');
    expect(current(page)).toEqual({ pathname: PAGE, search: '' });
  });

  test('MP-36 an offline server puts its canonical host in the shared URL', async ({ page }) => {
    await mockMcStatus(page, () => ({
      status: 404,
      contentType: 'application/problem+json',
      body: JSON.stringify({ title: 'Not Found', status: 404, host: '2001:db8::1', port: 25566 }),
    }));
    await page.goto(PAGE);
    await lookup(page, '[2001:DB8:0:0:0:0:0:1]:25566');
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    expect(current(page).pathname).toBe(`${PAGE}/%5B2001:db8::1%5D:25566`);
  });

  test('MP-36 a failed lookup with another status does not rewrite the URL', async ({ page }) => {
    await mockMcStatus(page, () => ({ status: 500, contentType: 'application/problem+json', body: '{}' }));
    await page.goto(PAGE);
    await lookup(page, 'play.example.net');
    await expect(page.locator('#mc-status-error')).toBeVisible();
    expect(current(page).pathname).toBe(PAGE);
  });

  test('MP-37 an error replaces a previous result, and a later success replaces the error', async ({ page }) => {
    let fail = false;
    await mockMcStatus(page, () => (fail ? problem(404, 'down') : json(ONLINE)));
    await page.goto(PAGE);
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

  test('MP-38 a lookup that never answers times out with a message and frees the button', async ({ page }) => {
    await page.clock.install();
    await page.route(`${API}/mcstatus/**`, () => new Promise(() => {}));
    await page.goto(PAGE);
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

  test('MP-38 a lookup whose error body never finishes times out with the same message', async ({ page }) => {
    await page.clock.install();
    await page.addInitScript(() => {
      const realFetch = window.fetch.bind(window);
      window.fetch = (input, init) => {
        if (!String(input).includes('/mcstatus/stalled')) {
          return realFetch(input, init);
        }
        const body = new ReadableStream({
          start(controller) {
            init.signal.addEventListener('abort', () => controller.error(new DOMException('Aborted', 'AbortError')));
          },
        });
        return Promise.resolve(new Response(body, { status: 500, headers: { 'content-type': 'application/problem+json' } }));
      };
    });
    await page.goto(PAGE);
    await page.clock.pauseAt(new Date(Date.now() + 1000));
    await lookup(page, 'stalled.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');

    await page.clock.runFor(30_000);
    await expect(page.locator('#mc-status-error-message')).toHaveText("Couldn't reach that server");
    await expect(page.locator('#mc-status-error-detail')).toHaveText('The lookup timed out.');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });
});

test.describe('superseded lookups', () => {
  test('MP-39 a superseded lookup whose error body arrives late does not show its error', async ({ page }) => {
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

    await page.goto(PAGE);
    await lookup(page, 'slow404');
    await expect.poll(() => page.evaluate(() => typeof window.__finishSlow404)).toBe('function');
    await lookup(page, 'hang.example.net');

    await page.evaluate(() => window.__finishSlow404());
    await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await expect(page.locator('#mc-status-error')).toBeHidden();
  });

  test('MP-40 a superseded lookup leaves the button and banner to the newest one', async ({ page }) => {
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

    await page.goto(PAGE);
    await lookup(page, 'old.example.net');
    await lookup(page, 'new.example.net');
    await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await expect(page.locator('#mc-status-error')).toBeHidden();

    releaseNewest();
    await expect(page.locator('#mc-status-name')).toHaveText('new.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
  });

  test('MP-41 a slow earlier lookup cannot overwrite a newer one', async ({ page }) => {
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

    await page.goto(PAGE);
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

  test('MP-42 aborting a lookup cancels its request, frees the button and its late answer shows nothing, and aborting with none is harmless', async ({ page }) => {
    let release;
    const released = new Promise((resolve) => {
      release = resolve;
    });
    await page.route(`${API}/mcstatus/held.example.net*`, async (route) => {
      await released;
      await route.fulfill(withTarget(json(ONLINE), route.request().url())).catch(() => {});
    });
    const thrown = [];
    page.on('pageerror', (failure) => thrown.push(failure.message));
    const failed = [];
    page.on('requestfailed', (request) => {
      if (request.url().includes('/mcstatus/')) {
        failed.push({ url: request.url(), reason: request.failure().errorText });
      }
    });
    await page.goto(PAGE);
    await page.evaluate(() => abortMcStatus());
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
    expect(failed).toEqual([]);

    await lookup(page, 'held.example.net');
    await expect(page.locator('#mc-status-submit')).toHaveText('Checking...');
    await page.evaluate(() => abortMcStatus());
    await expect(page.locator('#mc-status-submit')).toHaveText('Check');
    await expect.poll(() => failed.map((entry) => entry.reason)).toEqual(['net::ERR_ABORTED']);
    expect(failed[0].url).toContain('/mcstatus/held.example.net');
    release();
    await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 100)));
    await expect(page.locator('#mc-status-result')).toBeHidden();
    await expect(page.locator('#mc-status-error')).toBeHidden();
    expect(thrown).toEqual([]);
  });
});

test.describe('share links', () => {
  for (const path of [PAGE, `${PAGE}?host=play.example.net`, `${PAGE}?bedrock=true&query=false&query_port=25575`]) {
    test(`MP-50 ${path} runs no lookup and leaves the form at its defaults`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(path);
      await expect(page.locator('#mc-status-host')).toBeVisible();
      await expect(page.locator('#mc-status-host')).toHaveValue('');
      await expect(page.locator('input[name="mc-edition"][value="java"]')).toBeChecked();
      await expect(page.locator('#mc-status-query')).toBeChecked();
      await expect(page.locator('#mc-status-advanced')).not.toHaveAttribute('open', '');
      await expect(page.locator('#mc-status-result')).toBeHidden();
      await expect(page.locator('#mc-status-error')).toBeHidden();
      expect(requests.status).toHaveLength(0);
    });
  }

  const hostsInPath = {
    'a host': ['play.example.net', 'play.example.net', 'play.example.net', `${API_PATH}/mcstatus/play.example.net`, `${PAGE}/play.example.net`],
    'a host with an escaped port': ['play.example.net%3A25570', 'play.example.net:25570', 'play.example.net:25570', `${API_PATH}/mcstatus/play.example.net%3A25570`, `${PAGE}/play.example.net:25570`],
    'an IPv6 host with escaped brackets': ['%5B2001:db8::1%5D:25566', '[2001:db8::1]:25566', '[2001:db8::1]:25566', `${API_PATH}/mcstatus/%5B2001%3Adb8%3A%3A1%5D%3A25566`, `${PAGE}/%5B2001:db8::1%5D:25566`],
    'an IPv6 host with unescaped brackets': ['[2001:db8::1]', '[2001:db8::1]', '[2001:db8::1]', `${API_PATH}/mcstatus/%5B2001%3Adb8%3A%3A1%5D`, `${PAGE}/%5B2001:db8::1%5D`],
  };

  for (const [name, [path, field, shown, apiPath, shared]] of Object.entries(hostsInPath)) {
    test(`MP-51 ${name} in the path fills the form, runs the lookup as typed and keeps the shared URL readable`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(`${PAGE}/${path}`);
      await expect(page.locator('#mc-status-result')).toBeVisible();
      await expect(page.locator('#mc-status-host')).toHaveValue(field);
      await expect(page.locator('#mc-status-name')).toHaveText(shown);
      expect(requests.status).toHaveLength(1);
      expect(requests.status[0].pathname).toBe(apiPath);
      expect(current(page).pathname).toBe(shared);
    });
  }

  const queries = {
    '?bedrock=true selects Bedrock and skips the query': {
      search: '?bedrock=true',
      edition: 'bedrock',
      query: false,
      sent: { bedrock: 'true', query: null, query_port: null },
      port: '',
      advanced: false,
    },
    '?query=false leaves the query box unticked': {
      search: '?query=false',
      edition: 'java',
      query: false,
      sent: { bedrock: null, query: null, query_port: null },
      port: '',
      advanced: false,
    },
    'a valid ?query_port= fills the field, opens Advanced and is sent': {
      search: '?query_port=25575',
      edition: 'java',
      query: true,
      sent: { bedrock: null, query: 'true', query_port: '25575' },
      port: '25575',
      advanced: true,
    },
    'an invalid ?query_port= is dropped from the field, the request and the URL': {
      search: '?query_port=0',
      edition: 'java',
      query: true,
      sent: { bedrock: null, query: 'true', query_port: null },
      port: '',
      advanced: false,
      urlSearch: '',
    },
  };

  for (const [name, { search, edition, query, sent, port, advanced, urlSearch }] of Object.entries(queries)) {
    test(`MP-52 ${name}`, async ({ page }) => {
      const requests = await mockMcStatus(page, () => json(ONLINE));
      await page.goto(`${PAGE}/play.example.net${search}`);
      await expect(page.locator('#mc-status-result')).toBeVisible();
      await expect(page.locator(`input[name="mc-edition"][value="${edition}"]`)).toBeChecked();
      if (query) {
        await expect(page.locator('#mc-status-query')).toBeChecked();
      } else {
        await expect(page.locator('#mc-status-query')).not.toBeChecked();
      }
      await expect(page.locator('#mc-status-query-port')).toHaveValue(port);
      await expect(page.locator('#mc-status-advanced')).toHaveJSProperty('open', advanced);
      for (const [key, value] of Object.entries(sent)) {
        expect(requests.status[0].searchParams.get(key), key).toBe(value);
      }
      expect(current(page)).toEqual({ pathname: `${PAGE}/play.example.net`, search: urlSearch ?? search });
    });
  }

  test('MP-53 a path that cannot be decoded runs no lookup and leaves the form alone', async ({ page }) => {
    const requests = await mockMcStatus(page, () => json(ONLINE));
    const thrown = [];
    page.on('pageerror', (failure) => thrown.push(failure.message));
    await page.goto(`${PAGE}/%C0%AF`);
    await expect(page.locator('#mc-status-host')).toBeVisible();
    await expect(page.locator('#mc-status-host')).toHaveValue('');
    await expect(page.locator('#mc-status-result')).toBeHidden();
    await expect(page.locator('#mc-status-error')).toBeHidden();
    expect(requests.status).toHaveLength(0);
    expect(thrown).toEqual([]);
  });
});
