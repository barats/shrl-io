import { createHmac, timingSafeEqual } from 'node:crypto';
import { config } from './config';

export const SESSION_COOKIE = 'shrl_session';

export interface SessionData {
	token: string;
	username: string;
	isAdmin: boolean;
	exp: number;
}

function sign(value: string): string {
	return createHmac('sha256', config.sessionSecret).update(value).digest('hex');
}

function encode(data: SessionData): string {
	return Buffer.from(JSON.stringify(data)).toString('base64url');
}

function decode(encoded: string): SessionData | null {
	try {
		const data = JSON.parse(Buffer.from(encoded, 'base64url').toString('utf8')) as SessionData;
		if (typeof data.token !== 'string' || typeof data.exp !== 'number') return null;
		return data;
	} catch {
		return null;
	}
}

export function createSessionCookie(data: SessionData): string {
	const payload = encode(data);
	const value = `${payload}.${sign(payload)}`;
	const secure = config.secureCookie ? '; Secure' : '';
	return `${SESSION_COOKIE}=${value}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${config.sessionTtlSeconds}${secure}`;
}

export function readSession(cookieHeader: string | null): SessionData | null {
	if (!cookieHeader) return null;
	const value = getCookie(cookieHeader, SESSION_COOKIE);
	if (!value) return null;
	const dot = value.lastIndexOf('.');
	if (dot <= 0 || dot === value.length - 1) return null;
	const payload = value.slice(0, dot);
	const sig = value.slice(dot + 1);
	const expected = sign(payload);
	const a = Buffer.from(sig);
	const b = Buffer.from(expected);
	if (a.length !== b.length || !timingSafeEqual(a, b)) return null;
	const data = decode(payload);
	if (!data) return null;
	if (data.exp < Date.now() / 1000) return null;
	return data;
}

export function clearSessionCookie(): string {
	return `${SESSION_COOKIE}=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0`;
}

function getCookie(header: string, name: string): string | undefined {
	for (const part of header.split(';')) {
		const eq = part.indexOf('=');
		if (eq === -1) continue;
		if (part.slice(0, eq).trim() === name) return part.slice(eq + 1).trim();
	}
	return undefined;
}
