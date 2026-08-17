const boot = window.__POOL_BOOTSTRAP__;
let principal = boot.principal;

async function request(path, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set('Accept', 'application/json');
  if (options.body && typeof options.body !== 'string') {
    headers.set('Content-Type', 'application/json');
    options.body = JSON.stringify(options.body);
  }
  if (options.method && options.method !== 'GET') headers.set('X-CSRF-Token', principal.csrfToken);
  const res = await fetch(path, {...options, headers, credentials:'same-origin'});
  let data = {};
  try { data = await res.json(); } catch {}
  if (!res.ok) {
    const err = new Error(data.error || `HTTP ${res.status}`);
    err.code = data.error || 'http_error'; err.status = res.status; throw err;
  }
  if (data.principal) principal = data.principal;
  return data;
}
export const api = {
  get principal(){ return principal; },
  session:()=>request('/api/session'),
  guest:nickname=>request('/api/guest',{method:'POST',body:{nickname}}),
  register:body=>request('/api/register',{method:'POST',body}),
  login:body=>request('/api/login',{method:'POST',body}),
  logout:()=>request('/api/logout',{method:'POST',body:{}}),
  lobbies:()=>request('/api/lobbies'),
  createLobby:body=>request('/api/lobbies',{method:'POST',body}),
  ticket:(code,body)=>request(`/api/lobbies/${encodeURIComponent(code)}/ticket`,{method:'POST',body}),
  profile:()=>request('/api/profile'),
  matches:()=>request('/api/matches'),
};
