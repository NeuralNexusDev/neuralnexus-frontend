const ID = {
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

const PERMISSIONS = [
  { id: ID.pBee, node: 'beenamegenerator.admin', description: 'Bee name generator' },
  { id: ID.pRate, node: 'ratelimit', description: 'Rate limit', value_type: 'int', merge: 'max' },
  { id: ID.pPets, node: 'petpictures.pets', description: 'Pet pictures', value_type: 'string_list', merge: 'union' },
  { id: ID.pMotd, node: 'motd', description: 'Message of the day', value_type: 'string', merge: 'first' },
  { id: ID.pStore, node: 'datastore.admin', description: 'Data store' },
];

function defaultState() {
  return {
    me: ['users.admin', 'roles.admin'],
    permissions: structuredClone(PERMISSIONS),
    roles: [
      { id: ID.system, name: 'system', description: 'System', grants: [] },
      { id: ID.owner, name: 'owner', description: 'Owner', grants: [] },
      { id: ID.bee, name: 'bee_admin', description: 'Bee Name Generator Admin', grants: [{ id: ID.pBee }, { id: ID.pRate, value: 100 }] },
    ],
    users: [
      { user_id: ID.alice, username: 'alice', roles: [ID.system] },
      { user_id: ID.bob, username: 'bob', roles: [ID.bee] },
      { user_id: ID.anon, username: '', roles: [] },
    ],
    links: {
      [ID.bob]: [
        { platform: 'discord', platform_username: 'bob#1234', platform_id: '9' },
        { platform: 'steam', platform_id: '76561198000000000' },
      ],
    },
    account: { username: 'testuser', password_auth: true },
    myLinks: [],
    suggestions: ['buzz', 'honey', 'wax'],
    failures: {},
    delays: {},
    gates: {},
    nextId: '3541025163146800000',
    calls: [],
  };
}

const sessions = new Map();

function stateFor(session) {
  if (!sessions.has(session)) {
    sessions.set(session, defaultState());
  }
  return sessions.get(session);
}

function problem(res, status, detail) {
  res.writeHead(status, { 'Content-Type': 'application/problem+json' }).end(JSON.stringify({ title: 'x', status, detail }));
}

function send(res, status, body) {
  if (body === undefined) {
    res.writeHead(status).end();
    return;
  }
  res.writeHead(status, { 'Content-Type': 'application/json' }).end(JSON.stringify(body));
}

function roleView(state, role) {
  return {
    id: role.id,
    name: role.name,
    description: role.description,
    permissions: role.grants.map((grant) => {
      const permission = state.permissions.find((candidate) => candidate.id === grant.id);
      return grant.value === undefined ? { ...permission } : { ...permission, value: grant.value };
    }),
  };
}

function effective(state, user) {
  const ints = new Map();
  const others = new Map();
  for (const roleId of user.roles) {
    const role = state.roles.find((candidate) => candidate.id === roleId);
    for (const grant of role ? role.grants : []) {
      const permission = state.permissions.find((candidate) => candidate.id === grant.id);
      if (grant.value === undefined) {
        others.set(permission.node, permission.node);
      } else if (permission.value_type === 'int') {
        ints.set(permission.node, Math.max(ints.get(permission.node) ?? grant.value, grant.value));
      } else {
        const value = Array.isArray(grant.value) ? grant.value.join(',') : grant.value;
        others.set(permission.node, `${permission.node}:${value}`);
      }
    }
  }
  return [...others.values(), ...[...ints].map(([node, value]) => `${node}:${value}`)].sort();
}

function validValue(permission, value) {
  switch (permission.value_type) {
    case 'int':
      return Number.isSafeInteger(value);
    case 'string':
      return typeof value === 'string' && value.trim() === value && value !== '';
    case 'string_list':
      return Array.isArray(value) && value.length > 0 && value.every((item) => typeof item === 'string' && item.trim() === item && item !== '');
    default:
      return value === undefined;
  }
}

function readBody(req) {
  return new Promise((resolve) => {
    let raw = '';
    req.on('data', (chunk) => (raw += chunk));
    req.on('end', () => {
      try {
        resolve(raw ? JSON.parse(raw) : undefined);
      } catch {
        resolve(null);
      }
    });
  });
}

const BUILTIN = ['system', 'owner'];
const ROLE_NAME = /^[a-z][a-z0-9_]*$/;
const NODE = /^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$/;

/**
 * Serves /api/v1/users, /roles and /permissions from per-session state; the test seeds it through /__admin/state.
 * A seeded failure answers "METHOD /path" with its status and detail after letting `skip` calls through, `times` times (default always).
 * A seeded delay holds the answer to "METHOD /path" for that many milliseconds, which only the htmx timeout specs need.
 * A gate holds the answer to "METHOD /path" until the test releases it, after letting `skip` calls through.
 * Releasing with an index answers only the call that arrived in that position.
 */
export async function handleState(req, res, pathname, searchParams) {
  const cookie = /(?:^|;\s*)session=([^;]+)/.exec(req.headers.cookie || '');
  if (!cookie) {
    return problem(res, 401, 'Sign in to continue');
  }
  const state = stateFor(decodeURIComponent(cookie[1]));
  const path = pathname.slice('/api/v1'.length);
  const parts = path.split('/').filter(Boolean).map(decodeURIComponent);
  const body = req.method === 'GET' || req.method === 'DELETE' ? undefined : await readBody(req);
  state.calls.push({ method: req.method, path, query: searchParams.toString(), body });

  const gate = state.gates[`${req.method} ${path}`];
  if (gate && !gate.open) {
    if (gate.skip > 0) {
      gate.skip -= 1;
    } else {
      gate.arrived += 1;
      const outcome = await new Promise((resolve) => gate.held.push(resolve));
      if (outcome.status) {
        return problem(res, outcome.status, outcome.detail);
      }
    }
  }
  const delay = state.delays[`${req.method} ${path}`];
  if (delay) {
    await new Promise((resolve) => setTimeout(resolve, delay));
  }
  const failure = state.failures[`${req.method} ${path}`];
  if (failure && failure.skip > 0) {
    failure.skip -= 1;
  } else if (failure && failure.times !== 0) {
    if (failure.times > 0) {
      failure.times -= 1;
    }
    return problem(res, failure.status, failure.detail);
  }

  const [kind, id, sub, subId] = parts;
  const can = (node) => state.me.some((permission) => permission === node || permission.startsWith(`${node}:`));

  if (kind === 'bee-name-generator') {
    if (!can('beenamegenerator.admin')) {
      return problem(res, 403, 'You do not have permission to review suggestions');
    }
    if (req.method === 'GET') {
      return send(res, 200, { suggestions: state.suggestions.slice(0, Number(sub)) });
    }
    if (req.method === 'POST') {
      state.suggestions.push(sub);
      return send(res, 200);
    }
    state.suggestions = state.suggestions.filter((name) => name !== sub);
    return req.method === 'DELETE' ? send(res, 204) : send(res, 200);
  }
  if (path === '/users/me/permissions') {
    return send(res, 200, state.me);
  }
  if (kind === 'users' && id === 'me') {
    if (!sub && req.method === 'GET') {
      return send(res, 200, { username: state.account.username });
    }
    if (sub === 'settings' && req.method === 'GET') {
      return send(res, 200, { password_auth: state.account.password_auth });
    }
    if (sub === 'settings' && req.method === 'PATCH') {
      state.account.password_auth = body.password_auth;
      return send(res, 204);
    }
    if (sub === 'links' && req.method === 'GET') {
      return send(res, 200, state.myLinks);
    }
    const link = state.myLinks.find((candidate) => candidate.platform === subId);
    if (sub === 'link' && !link) {
      return problem(res, 404, 'That platform is not linked');
    }
    if (sub === 'link' && req.method === 'PATCH') {
      link.login_enabled = body.login_enabled;
      return send(res, 204);
    }
    if (sub === 'link' && req.method === 'DELETE') {
      state.myLinks = state.myLinks.filter((candidate) => candidate !== link);
      return send(res, 204);
    }
  }
  if (kind === 'users') {
    if (!can('users.admin')) {
      return problem(res, 403, 'You do not have permission to manage users');
    }
    if (parts.length === 1) {
      const limit = Number(searchParams.get('limit') ?? 50);
      const offset = Number(searchParams.get('offset') ?? 0);
      if (!Number.isInteger(limit) || limit < 1 || limit > 200 || !Number.isInteger(offset) || offset < 0) {
        return problem(res, 400, 'The limit is from 1 to 200 and the offset from 0');
      }
      return send(res, 200, state.users.slice(offset, offset + limit));
    }
    const user = state.users.find((candidate) => candidate.user_id === id);
    if (!user) {
      return problem(res, 404, 'User not found');
    }
    if (sub === 'links') {
      return send(res, 200, state.links[id] || []);
    }
    if (sub === 'permissions') {
      return send(res, 200, effective(state, user));
    }
    if (req.method === 'PUT') {
      if (body === null || typeof body !== 'object') {
        return problem(res, 400, 'Invalid input, unable to parse body');
      }
      if (body.roles && !body.roles.every((roleId) => state.roles.some((role) => role.id === roleId))) {
        return problem(res, 400, 'Roles must be existing roles');
      }
      if (body.username && state.users.some((other) => other !== user && other.username === body.username)) {
        return problem(res, 409, 'An account with this username already exists');
      }
      if (body.username) {
        user.username = body.username;
      }
      if (body.roles) {
        user.roles = body.roles;
      }
    }
    return send(res, 200, user);
  }

  if (!can('roles.admin')) {
    return problem(res, 403, 'You do not have permission to manage roles and permissions');
  }
  if (kind === 'permissions') {
    if (parts.length === 1 && req.method === 'GET') {
      return send(res, 200, state.permissions);
    }
    if (parts.length === 1 && req.method === 'POST') {
      if (!body || !NODE.test(body.node || '')) {
        return problem(res, 400, 'Nodes are lower-case words of letters, digits and underscores, starting with a letter and joined by dots');
      }
      if (state.permissions.some((permission) => permission.node === body.node)) {
        return problem(res, 409, 'That permission already exists');
      }
      const created = { id: String((BigInt(state.nextId) + 1n)), node: body.node, description: body.description || '' };
      state.nextId = created.id;
      if (body.value_type) {
        created.value_type = body.value_type;
        created.merge = body.merge || (body.value_type === 'string_list' ? 'union' : 'first');
      }
      state.permissions.push(created);
      return send(res, 201, created);
    }
    if (req.method === 'DELETE') {
      if (state.roles.some((role) => role.grants.some((grant) => grant.id === id))) {
        return problem(res, 409, 'The permission is granted by a role');
      }
      state.permissions = state.permissions.filter((permission) => permission.id !== id);
      return send(res, 204);
    }
  }
  if (kind === 'roles') {
    if (parts.length === 1 && req.method === 'GET') {
      return send(res, 200, state.roles.map((role) => roleView(state, role)));
    }
    if (parts.length === 1 && req.method === 'POST') {
      if (!body || !ROLE_NAME.test(body.name || '')) {
        return problem(res, 400, 'Role names start with a lower-case letter and use only lower-case letters, digits and underscores');
      }
      if (state.roles.some((role) => role.name === body.name)) {
        return problem(res, 409, 'A role with that name already exists');
      }
      const created = { id: String(BigInt(state.nextId) + 1n), name: body.name, description: body.description || '', grants: [] };
      state.nextId = created.id;
      state.roles.push(created);
      return send(res, 201, roleView(state, created));
    }
    const role = state.roles.find((candidate) => candidate.id === id);
    if (!role) {
      return problem(res, 404, 'Role not found');
    }
    if (parts.length === 2) {
      if (req.method === 'PATCH') {
        if (body.name !== undefined && body.name !== role.name) {
          if (BUILTIN.includes(role.name)) {
            return problem(res, 409, 'Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin');
          }
          if (!ROLE_NAME.test(body.name)) {
            return problem(res, 400, 'Role names start with a lower-case letter and use only lower-case letters, digits and underscores');
          }
          if (state.roles.some((other) => other.name === body.name)) {
            return problem(res, 409, 'A role with that name already exists');
          }
          role.name = body.name;
        }
        if (body.description !== undefined) {
          role.description = body.description;
        }
      }
      if (req.method === 'DELETE') {
        if (BUILTIN.includes(role.name)) {
          return problem(res, 409, 'Built-in roles cannot be deleted or renamed, and system and owner keep roles.admin');
        }
        if (state.users.some((user) => user.roles.includes(role.id))) {
          return problem(res, 409, 'The role is assigned to an account');
        }
        state.roles = state.roles.filter((candidate) => candidate !== role);
        return send(res, 204);
      }
      return send(res, 200, roleView(state, role));
    }
    if (sub === 'permissions' && subId) {
      const permission = state.permissions.find((candidate) => candidate.id === subId);
      if (!permission) {
        return problem(res, 404, 'Permission not found');
      }
      const index = role.grants.findIndex((grant) => grant.id === subId);
      if (req.method === 'PUT') {
        const value = body ? body.value : undefined;
        if (!validValue(permission, value)) {
          return problem(res, 400, "The value must match the permission's type, and permissions without a type take no value");
        }
        const grant = value === undefined ? { id: subId } : { id: subId, value };
        if (index >= 0) {
          role.grants[index] = grant;
        } else {
          role.grants.push(grant);
        }
        return send(res, 204);
      }
      if (req.method === 'DELETE') {
        if (index >= 0) {
          role.grants.splice(index, 1);
        }
        return send(res, 204);
      }
    }
  }
  return problem(res, 404, 'Not found');
}

/** Seeds or reads the state of one test's session, which the app forwards to the API as its session cookie. */
export async function handleControl(req, res, pathname, searchParams) {
  if (pathname === '/__admin/state' && req.method === 'POST') {
    const { session, state } = await readBody(req);
    const seeded = { ...defaultState(), ...state };
    for (let i = 0; i < (state.generateUsers || 0); i += 1) {
      seeded.users.push({ user_id: String(3541025163146700000n + BigInt(i + 1)), username: `user${i + 1}`, roles: [] });
    }
    sessions.set(session, seeded);
    return send(res, 200, {});
  }
  if (pathname === '/__admin/gate' && req.method === 'POST') {
    const { session, key, skip } = await readBody(req);
    stateFor(session).gates[key] = { skip: skip || 0, arrived: 0, open: false, held: [] };
    return send(res, 200, {});
  }
  if (pathname === '/__admin/gate' && req.method === 'GET') {
    const gate = stateFor(searchParams.get('session')).gates[searchParams.get('key')];
    return send(res, 200, { arrived: gate ? gate.arrived : 0 });
  }
  if (pathname === '/__admin/gate/release' && req.method === 'POST') {
    const { session, key, status, detail, index } = await readBody(req);
    const gate = stateFor(session).gates[key];
    if (index !== undefined) {
      gate.held[index]({ status, detail });
      return send(res, 200, {});
    }
    gate.open = true;
    for (const resolve of gate.held.splice(0)) {
      resolve({ status, detail });
    }
    return send(res, 200, {});
  }
  if (pathname === '/__admin/calls' && req.method === 'GET') {
    return send(res, 200, stateFor(searchParams.get('session')).calls);
  }
  return false;
}
