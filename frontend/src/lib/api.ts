import type { AnalyticsResponse, BreakdownResponse, Link, TimeseriesRow, User } from './types';

async function req<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(`/api/${path}`, init);
	if (res.status === 401) {
		if (typeof window !== 'undefined') window.location.assign('/login');
		throw new Error('unauthorized');
	}
	if (!res.ok) {
		const text = await res.text().catch(() => '');
		let msg = `request failed (${res.status})`;
		try {
			msg = (JSON.parse(text) as { error?: string }).error ?? msg;
		} catch {
			if (text) msg = text;
		}
		throw new Error(msg);
	}
	if (res.status === 204) return undefined as T;
	return (await res.json()) as T;
}

export interface CreateLinkInput {
	hostname?: string;
	code?: string;
	destination: string;
}

export const api = {
	async config(): Promise<{ defaultHostname: string }> {
		return req<{ defaultHostname: string }>('config');
	},

	async hostnames(): Promise<string[]> {
		return req<string[]>('hostnames');
	},

	async listLinks(hostname?: string): Promise<Link[]> {
		const q = hostname ? `?hostname=${encodeURIComponent(hostname)}` : '';
		return req<Link[]>('links' + q);
	},

	async createLink(input: CreateLinkInput): Promise<Link> {
		return req<Link>('links', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify(input)
		});
	},

	async getLink(code: string, hostname: string): Promise<Link> {
		return req<Link>(
			`links/${encodeURIComponent(code)}?hostname=${encodeURIComponent(hostname)}`
		);
	},

	async updateLink(code: string, hostname: string, destination: string): Promise<Link> {
		return req<Link>(
			`links/${encodeURIComponent(code)}?hostname=${encodeURIComponent(hostname)}`,
			{
				method: 'PATCH',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ destination })
			}
		);
	},

	async setDisabled(code: string, hostname: string, disabled: boolean): Promise<Link> {
		const action = disabled ? 'disable' : 'enable';
		return req<Link>(
			`links/${encodeURIComponent(code)}/${action}?hostname=${encodeURIComponent(hostname)}`,
			{ method: 'POST' }
		);
	},

	async deleteLink(code: string, hostname: string): Promise<void> {
		return req<void>(
			`links/${encodeURIComponent(code)}?hostname=${encodeURIComponent(hostname)}`,
			{ method: 'DELETE' }
		);
	},

	async getAnalytics(code: string, hostname: string): Promise<AnalyticsResponse> {
		const q = new URLSearchParams({ hostname });
		return req<AnalyticsResponse>(`links/${encodeURIComponent(code)}/analytics?${q}`);
	},

	async getTimeseries(
		code: string,
		hostname: string,
		from?: string,
		to?: string
	): Promise<TimeseriesRow[]> {
		const q = new URLSearchParams({ hostname });
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<TimeseriesRow[]>(
			`links/${encodeURIComponent(code)}/analytics/timeseries?${q}`
		);
	},

	async getBreakdowns(
		code: string,
		hostname: string,
		dimension: string,
		from?: string,
		to?: string
	): Promise<BreakdownResponse> {
		const q = new URLSearchParams({ hostname, dimension });
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<BreakdownResponse>(
			`links/${encodeURIComponent(code)}/analytics/breakdowns?${q}`
		);
	},

	async login(username: string, password: string): Promise<void> {
		const res = await fetch('/api/login', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ username, password })
		});
		if (!res.ok) throw new Error('invalid username or password');
	},

	async logout(): Promise<void> {
		await fetch('/api/logout', { method: 'POST' });
	},

	async me(): Promise<User> {
		return req<User>('me');
	},

	async listUsers(): Promise<User[]> {
		return req<User[]>('users');
	},

	async createUser(input: {
		username: string;
		password?: string;
		is_admin?: boolean;
	}): Promise<{ user: User; password: string }> {
		return req<{ user: User; password: string }>('users', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify(input)
		});
	}
};

export function daysAgo(n: number): string {
	const d = new Date();
	d.setUTCDate(d.getUTCDate() - n);
	return d.toISOString().slice(0, 10);
}
