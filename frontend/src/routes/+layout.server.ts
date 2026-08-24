import { readSession } from '$lib/server/auth';

export function load({ request }) {
	const session = readSession(request.headers.get('cookie'));
	return {
		user: session ? { username: session.username, isAdmin: session.isAdmin } : null
	};
}
