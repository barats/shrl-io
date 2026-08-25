import { redirect } from '@sveltejs/kit';
import { readSession } from '$lib/server/auth';
import { config } from '$lib/server/config';

export async function load({ request, url }) {
	const session = readSession(request.headers.get('cookie'));
	// A user on a temporary password from an admin reset may only change their
	// password until it is replaced (ADR 0012). The session flag can go stale
	// the moment the user changes their password from /account, so when it is
	// set we refresh against the backend, which is the source of truth.
	let mustChangePassword = session?.mustChangePassword === true;
	if (mustChangePassword && session) {
		try {
			const res = await fetch(`${config.apiUrl}/me`, {
				headers: { authorization: `Bearer ${session.token}` }
			});
			const body = (await res.json().catch(() => null)) as { must_change_password?: boolean } | null;
			mustChangePassword = body?.must_change_password === true;
		} catch {
			/* backend unreachable: fall back to the session flag */
		}
	}
	if (mustChangePassword && url.pathname !== '/account') {
		throw redirect(303, '/account');
	}
	return {
		user: session ? { username: session.username, isAdmin: session.isAdmin } : null
	};
}
