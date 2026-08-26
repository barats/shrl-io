import { apiFetch } from '$lib/server/config';
import { clearSessionCookie, readSession } from '$lib/server/auth';

export async function POST({ request }) {
	const session = readSession(request.headers.get('cookie'));
	if (session) {
		await apiFetch('logout', {
			method: 'POST',
			headers: { authorization: `Bearer ${session.token}` }
		}).catch(() => {});
	}
	return new Response(JSON.stringify({ ok: true }), {
		status: 200,
		headers: { 'content-type': 'application/json', 'set-cookie': clearSessionCookie() }
	});
}
