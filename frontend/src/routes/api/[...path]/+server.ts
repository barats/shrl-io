import { config } from '$lib/server/config';
import { clearSessionCookie, readSession } from '$lib/server/auth';
import type { RequestHandler } from './$types';

async function proxy(request: Request, path: string): Promise<Response> {
	const session = readSession(request.headers.get('cookie'));
	if (!session) {
		return new Response(JSON.stringify({ error: 'unauthorized' }), {
			status: 401,
			headers: { 'content-type': 'application/json' }
		});
	}

	const url = new URL(request.url);
	const target = `${config.apiUrl}/${path}${url.search}`;

	const headers = new Headers();
	const contentType = request.headers.get('content-type');
	if (contentType) headers.set('content-type', contentType);
	headers.set('authorization', `Bearer ${session.token}`);

	const body =
		request.method === 'GET' || request.method === 'HEAD' ? undefined : await request.arrayBuffer();

	const res = await fetch(target, { method: request.method, headers, body });

	const responseHeaders = new Headers();
	const responseContentType = res.headers.get('content-type');
	if (responseContentType) responseHeaders.set('content-type', responseContentType);
	// A 401 means the session token was rejected (expired, revoked, or
	// replaced server-side). Clear the cookie so the login page does not see a
	// stale session and bounce the user straight back to '/'.
	if (res.status === 401) {
		const cleared = clearSessionCookie();
		if (cleared) responseHeaders.set('set-cookie', cleared);
	}

	return new Response(res.body, {
		status: res.status,
		headers: responseHeaders
	});
}

export const GET: RequestHandler = ({ params, request }) => proxy(request, params.path ?? '');
export const POST: RequestHandler = ({ params, request }) => proxy(request, params.path ?? '');
export const PATCH: RequestHandler = ({ params, request }) => proxy(request, params.path ?? '');
export const DELETE: RequestHandler = ({ params, request }) => proxy(request, params.path ?? '');
