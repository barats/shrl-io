import type {
	AnalyticsResponse,
	ApiKey,
	BreakdownResponse,
	InviteCode,
	Link,
	Settings,
	Stats,
	Team,
	TeamDetail,
	TeamMember,
	TeamRole,
	TimeseriesRow,
	User
} from './types';

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
		if (typeof window !== 'undefined' && res.status === 403 && msg === 'password change required') {
			// A temp-password user can only change their password until they do.
			window.location.assign('/account');
		}
		throw new Error(msg);
	}
	if (res.status === 204) return undefined as T;
	return (await res.json()) as T;
}

export interface CreateLinkInput {
	hostname?: string;
	destination: string;
	remark?: string;
	forward_utm?: boolean;
}

export const api = {
	async config(): Promise<{ defaultHostname: string }> {
		return req<{ defaultHostname: string }>('config');
	},

	async hostnames(): Promise<string[]> {
		return req<string[]>('hostnames');
	},

	async createHostname(name: string): Promise<unknown> {
		return req('hostnames', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ hostname: name })
		});
	},

	async deleteHostname(name: string): Promise<void> {
		return req(`hostnames/${encodeURIComponent(name)}`, { method: 'DELETE' });
	},

	async listLinks(): Promise<Link[]> {
		return req<Link[]>('links');
	},

	async createLink(input: CreateLinkInput): Promise<Link> {
		return req<Link>('links', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify(input)
		});
	},

	async getLink(code: string): Promise<Link> {
		return req<Link>(`links/${encodeURIComponent(code)}`);
	},

	async updateLink(
		code: string,
		destination: string,
		remark = '',
		forwardUtm = false
	): Promise<Link> {
		return req<Link>(
			`links/${encodeURIComponent(code)}`,
			{
				method: 'PATCH',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ destination, remark, forward_utm: forwardUtm })
			}
		);
	},

	async setDisabled(code: string, disabled: boolean): Promise<Link> {
		const action = disabled ? 'disable' : 'enable';
		return req<Link>(`links/${encodeURIComponent(code)}/${action}`, { method: 'POST' });
	},

	async deleteLink(code: string): Promise<void> {
		return req<void>(`links/${encodeURIComponent(code)}`, {
			method: 'DELETE'
		});
	},

	async getAnalytics(code: string): Promise<AnalyticsResponse> {
		return req<AnalyticsResponse>(`links/${encodeURIComponent(code)}/analytics`);
	},

	async getTimeseries(
		code: string,
		from?: string,
		to?: string
	): Promise<TimeseriesRow[]> {
		const q = new URLSearchParams();
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<TimeseriesRow[]>(`links/${encodeURIComponent(code)}/analytics/timeseries?${q}`);
	},

	async getBreakdowns(
		code: string,
		dimension: string,
		from?: string,
		to?: string
	): Promise<BreakdownResponse> {
		const q = new URLSearchParams({ dimension });
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<BreakdownResponse>(`links/${encodeURIComponent(code)}/analytics/breakdowns?${q}`);
	},

	async getStats(from?: string, to?: string): Promise<Stats> {
		const q = new URLSearchParams();
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<Stats>(`stats?${q}`);
	},

	async getTeamStats(id: number, from?: string, to?: string): Promise<Stats> {
		const q = new URLSearchParams();
		if (from) q.set('from', from);
		if (to) q.set('to', to);
		return req<Stats>(`teams/${id}/stats?${q}`);
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
	},

	async deleteUser(id: number): Promise<void> {
		return req<void>(`users/${id}`, { method: 'DELETE' });
	},

	// --- Teams ---

	async listTeams(): Promise<Team[]> {
		return req<Team[]>('teams');
	},

	async createTeam(name: string): Promise<Team> {
		return req<Team>('teams', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ name })
		});
	},

	async getTeam(id: number): Promise<TeamDetail> {
		return req<TeamDetail>(`teams/${id}`);
	},

	async listTeamLinks(id: number): Promise<Link[]> {
		return req<Link[]>(`teams/${id}/links`);
	},

	async createTeamLink(
		id: number,
		input: CreateLinkInput
	): Promise<Link> {
		return req<Link>(`teams/${id}/links`, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify(input)
		});
	},

	async addTeamMember(id: number, username: string): Promise<TeamMember> {
		return req<TeamMember>(`teams/${id}/members`, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ username })
		});
	},

	async setTeamMemberRole(id: number, userId: number, role: TeamRole): Promise<TeamMember> {
		return req<TeamMember>(`teams/${id}/members/${userId}`, {
			method: 'PATCH',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ role })
		});
	},

	async removeTeamMember(id: number, userId: number): Promise<void> {
		return req<void>(`teams/${id}/members/${userId}`, { method: 'DELETE' });
	},

	async deleteTeam(id: number): Promise<void> {
		return req<void>(`teams/${id}`, { method: 'DELETE' });
	},

	// --- Invite codes ---

	async createInvite(id: number): Promise<InviteCode> {
		return req<InviteCode>(`teams/${id}/invites`, { method: 'POST' });
	},

	async listInvites(id: number): Promise<InviteCode[]> {
		return req<InviteCode[]>(`teams/${id}/invites`);
	},

	async revokeInvite(id: number, code: string): Promise<void> {
		return req<void>(`teams/${id}/invites/${encodeURIComponent(code)}`, { method: 'DELETE' });
	},

	async joinTeam(code: string): Promise<Team> {
		return req<Team>('teams/join', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ code })
		});
	},

	// --- Account ---

	async changePassword(currentPassword: string, newPassword: string): Promise<void> {
		return req('account/password', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
		});
	},

	async listApiKeys(): Promise<ApiKey[]> {
		return req<ApiKey[]>('keys');
	},

	async createApiKey(name: string): Promise<{ api_key: ApiKey; secret: string }> {
		return req<{ api_key: ApiKey; secret: string }>('keys', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ name })
		});
	},

	async revokeApiKey(id: number): Promise<void> {
		return req<void>(`keys/${id}`, { method: 'DELETE' });
	},

	async resetUserPassword(id: number): Promise<{ password: string }> {
		return req<{ password: string }>(`users/${id}/reset`, { method: 'POST' });
	},

	// --- Settings ---

	async getSettings(): Promise<Settings> {
		return req<Settings>('settings');
	},

	async updateCodeLength(codeLength: number): Promise<Settings> {
		return req<Settings>('settings/code-length', {
			method: 'PATCH',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ code_length: codeLength })
		});
	}
};

export function daysAgo(n: number): string {
	const d = new Date();
	d.setUTCDate(d.getUTCDate() - n);
	return d.toISOString().slice(0, 10);
}
