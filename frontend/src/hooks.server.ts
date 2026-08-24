import { redirect, type Handle } from '@sveltejs/kit';
import { readSession } from '$lib/server/auth';

export const handle: Handle = async ({ event, resolve }) => {
	const { pathname } = event.url;

	const isPublic =
		pathname === '/login' ||
		pathname === '/api/login' ||
		pathname === '/api/logout' ||
		pathname === '/robots.txt' ||
		pathname === '/favicon.ico' ||
		pathname.startsWith('/static/') ||
		pathname.startsWith('/_app/');

	if (!isPublic && !readSession(event.request.headers.get('cookie'))) {
		if (pathname.startsWith('/api/')) {
			return new Response(JSON.stringify({ error: 'unauthorized' }), {
				status: 401,
				headers: { 'content-type': 'application/json' }
			});
		}
		throw redirect(303, '/login');
	}

	return resolve(event);
};
