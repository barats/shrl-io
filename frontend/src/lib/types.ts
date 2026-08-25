export interface Link {
	hostname: string;
	code: string;
	destination: string;
	remark: string;
	disabled: boolean;
	created_by: number;
	team_id: number | null;
	created_at: string;
	updated_at: string;
}

export type TeamRole = 'owner' | 'member';

// Team as returned by GET /teams: role is the caller's role, empty for an
// admin viewing a team they do not belong to.
export interface Team {
	id: number;
	name: string;
	created_by: number;
	created_at: string;
	role: TeamRole | '';
}

export interface TeamMember {
	id: number;
	username: string;
	role: TeamRole;
	joined_at: string;
}

// Team detail as returned by GET /teams/{id}: adds the member list.
export interface TeamDetail {
	id: number;
	name: string;
	created_by: number;
	created_at: string;
	members: TeamMember[];
}

export interface InviteCode {
	id: number;
	team_id: number;
	code: string;
	created_by: number;
	created_at: string;
	used_by: number | null;
	used_at: string | null;
}

export interface User {
	id: number;
	username: string;
	is_admin: boolean;
	created_at: string;
}

export interface AnalyticsResponse {
	hostname: string;
	code: string;
	window_days: number;
	lifetime: { visits: number };
	window: { visits: number; unique_visitors: number };
}

export interface TimeseriesRow {
	hostname: string;
	code: string;
	day: string;
	visits: number;
	unique_visitors: number;
	created_at?: string;
	updated_at?: string;
}

export interface BreakdownResponse {
	dimension: string;
	total: number;
	items: Record<string, number>[];
	other: number;
}

export const DIMENSIONS = [
	'referrer',
	'device',
	'os',
	'browser',
	'country',
	'region',
	'city'
] as const;

export type Dimension = (typeof DIMENSIONS)[number];
