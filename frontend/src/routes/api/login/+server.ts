import { apiFetch, config } from '$lib/server/config';
import { createSessionCookie } from '$lib/server/auth';

export async function POST({ request }) {
	const body = await request.json().catch(() => null);
	const username = typeof body?.username === 'string' ? body.username.trim() : '';
	const password = typeof body?.password === 'string' ? body.password : '';
	if (!username || !password) {
		return new Response(JSON.stringify({ error: 'username and password are required' }), {
			status: 400,
			headers: { 'content-type': 'application/json' }
		});
	}

	const res = await apiFetch('login', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ username, password })
	});
	if (!res.ok) {
		return new Response(JSON.stringify({ error: 'invalid username or password' }), {
			status: 401,
			headers: { 'content-type': 'application/json' }
		});
	}

	const data = await res.json();
	const cookie = createSessionCookie({
		token: data.token,
		username: data.user.username,
		isAdmin: data.user.is_admin,
		mustChangePassword: data.user.must_change_password === true,
		exp: Math.floor(Date.now() / 1000) + config.sessionTtlSeconds
	});

	return new Response(JSON.stringify({ ok: true }), {
		status: 200,
		headers: { 'content-type': 'application/json', 'set-cookie': cookie }
	});
}
