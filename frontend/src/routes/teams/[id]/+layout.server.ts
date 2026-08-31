import { error, redirect } from '@sveltejs/kit';
import { readSession } from '$lib/server/auth';
import { apiFetch } from '$lib/server/config';

export async function load({ params, request }) {
	const session = readSession(request.headers.get('cookie'));
	if (!session) {
		throw redirect(303, '/login');
	}
	const res = await apiFetch(`teams/${params.id}`, {
		headers: { authorization: `Bearer ${session.token}` }
	});
	if (res.status === 404) {
		// Outsiders get 404 so team existence is not leaked.
		throw error(404, 'Team not found');
	}
	if (!res.ok) {
		throw error(500, 'Failed to load team');
	}
	const team = (await res.json()) as {
		id: number;
		name: string;
		members: { id: number; username: string; role: string; joined_at: string }[];
	};
	// The caller's role in this team, derived from the members list so the
	// team navbar can gate owner-only actions without an extra request.
	const myRole = team.members.find((m) => m.username === session.username)?.role ?? null;
	return { team, myRole };
}
