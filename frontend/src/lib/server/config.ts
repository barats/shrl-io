import { env } from '$env/dynamic/private';

function randomSecret(): string {
	return Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
}

export const config = {
	apiUrl: (env.SHRL_API_URL ?? 'http://localhost:8081').replace(/\/+$/, ''),
	defaultHostname: env.SHRL_DEFAULT_HOSTNAME ?? 'localhost',
	sessionSecret: env.SHRL_SESSION_SECRET || randomSecret(),
	sessionTtlSeconds: Number(env.SHRL_SESSION_TTL ?? 60 * 60 * 24),
	secureCookie: env.SHRL_COOKIE_SECURE === 'true'
};
