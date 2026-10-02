import http from 'node:http';

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

http
  .createServer((req, res) => {
    const { pathname } = new URL(req.url, 'http://stub');
    const host = decodeURIComponent(pathname.split('/').pop());
    if (pathname === '/health') {
      res.writeHead(200).end('ok');
    } else if (pathname.startsWith('/api/v1/mcstatus/icon/')) {
      res.writeHead(200, { 'Content-Type': 'image/png' }).end(PNG_1X1);
    } else if (host === 'online.example.net') {
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(ONLINE));
    } else {
      res.writeHead(404, { 'Content-Type': 'application/problem+json' }).end(JSON.stringify({ title: 'Not Found', status: 404 }));
    }
  })
  .listen(Number(process.env.PORT || 8098));
