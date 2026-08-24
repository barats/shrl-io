import { redirect } from '@sveltejs/kit';
import { readSession } from '$lib/server/auth';

export function load({ request }) {
	if (readSession(request.headers.get('cookie'))) {
		throw redirect(303, '/');
	}
}
