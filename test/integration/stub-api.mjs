import http from 'node:http';
import net from 'node:net';

const PNG_1X1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
  'base64'
);

const ONLINE = {
  name: 'Stub Network',
  motd: '§aHello\\n§c§lWorld',
  version: 'Paper 1.21',
  num_players: 3,
  max_players: 20,
  players: [{ name: 'Alex' }, { name: 'Steve' }],
};

const INCOMPLETE = /^(online|offline)-no-(host|port)\.example\.net$/;

const ONLINE_HOSTS = ['online.example.net', '2001:db8::1'];

const BAD_HOST_DETAIL = 'The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port.';

const DOMAIN = /^(?=.{1,253}$)(?:[a-z0-9_](?:[a-z0-9_-]{0,61}[a-z0-9_])?\.)+[a-z0-9_](?:[a-z0-9_-]{0,61}[a-z0-9_])?$/i;

const isIPv6 = (text) => net.isIPv6(text) && !text.includes('%');

function canonicalTarget(raw, bedrock) {
  let host = raw;
  let portText;
  const bracketed = /^\[([^\]]*)\](?::([^:]*))?$/.exec(raw);
  if (bracketed) {
    [, host, portText] = bracketed;
    if (!isIPv6(host)) {
      return null;
    }
  } else if (!isIPv6(raw)) {
    const plain = /^([^:]*)(?::([^:]*))?$/.exec(raw);
    if (!plain) {
      return null;
    }
    [, host, portText] = plain;
    if (!net.isIPv4(host) && !DOMAIN.test(host)) {
      return null;
    }
  }
  let port = bedrock ? 19132 : 25565;
  if (portText !== undefined) {
    port = /^[0-9]{1,5}$/.test(portText) ? Number(portText) : 0;
    if (port < 1 || port > 65535) {
      return null;
    }
  }
  if (isIPv6(host)) {
    host = new URL(`http://[${host}]`).hostname.slice(1, -1);
  }
  return { host: host.toLowerCase(), port };
}

function problem(res, status, body, contentType = 'application/problem+json') {
  res.writeHead(status, { 'Content-Type': contentType }).end(JSON.stringify({ status, ...body }));
}

const received = [];

http
  .createServer((req, res) => {
    const { pathname, searchParams } = new URL(req.url, 'http://stub');
    if (pathname === '/health') {
      res.writeHead(200).end('ok');
      return;
    }
    if (pathname === '/__requests') {
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(received));
      return;
    }
    received.push(req.url);
    const isIcon = pathname.startsWith('/api/v1/mcstatus/icon/');
    const target = canonicalTarget(decodeURIComponent(pathname.split('/').pop()), searchParams.get('bedrock') === 'true');
    if (!target) {
      problem(res, 400, { title: 'Bad Request', detail: BAD_HOST_DETAIL });
    } else if (isIcon) {
      res.writeHead(200, { 'Content-Type': 'image/png' }).end(PNG_1X1);
    } else if (INCOMPLETE.test(target.host)) {
      const [, state, missing] = INCOMPLETE.exec(target.host);
      const { [missing]: _omitted, ...partial } = target;
      if (state === 'online') {
        res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ ...ONLINE, ...partial }));
      } else {
        problem(res, 404, { title: 'Not Found', ...partial });
      }
    } else if (ONLINE_HOSTS.includes(target.host)) {
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ ...ONLINE, ...target }));
    } else {
      problem(res, 404, { title: 'Not Found', ...target });
    }
  })
  .listen(Number(process.env.PORT || 8098));
