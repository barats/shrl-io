import { env } from '$env/dynamic/private';

function randomSecret(): string {
	return Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
}

export const config = {
	apiUrl: (env.SHRL_API_URL ?? 'http://localhost:8080').replace(/\/+$/, ''),
	defaultBaseURL: env.SHRL_DEFAULT_BASE_URL ?? 'http://localhost:8080',
	// The shared secret the Internal API demands on every request (ADR 0015).
	// Both sides must agree; in production set SHRL_API_INTERNAL_SECRET on the
	// api and the frontend to the same value.
	internalSecret: env.SHRL_API_INTERNAL_SECRET || 'dev-internal-secret',
	sessionSecret: env.SHRL_SESSION_SECRET || randomSecret(),
	sessionTtlSeconds: Number(env.SHRL_SESSION_TTL ?? 60 * 60 * 24),
	secureCookie: env.SHRL_COOKIE_SECURE === 'true'
};

// apiFetch calls the Internal API with the shared secret header. Only the
// frontend server calls this; the secret never reaches the browser.
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const headers = new Headers(init.headers);
	headers.set('X-Shrl-Internal-Secret', config.internalSecret);
	return fetch(`${config.apiUrl}/${path}`, { ...init, headers });
}
