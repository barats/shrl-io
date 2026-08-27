export interface Link {
	hostname: string;
	code: string;
	destination: string;
	remark: string;
	disabled: boolean;
	forward_utm: boolean;
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
	must_change_password: boolean;
	created_at: string;
}

export interface ApiKey {
	id: number;
	name: string;
	created_at: string;
}

export interface Settings {
	code_length: number;
}

export interface AnalyticsResponse {
	code: string;
	window_days: number;
	lifetime: { visits: number };
	window: { visits: number; unique_visitors: number };
}

export interface TimeseriesRow {
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

// Aggregate analytics across a user's or team's links (GET /stats).
export interface Stats {
	total_links: number;
	total_visits: number;
	window_visits: number;
	window_uniques: number;
	timeseries: TimeseriesRow[];
}

// One link's window totals on the dashboard (top links rankings).
export interface DashboardTopLink {
	code: string;
	hostname: string;
	visits: number;
	unique_visitors: number;
}

// One dimension value's window totals (environment/location breakdowns).
export interface DashboardBreakdownItem {
	value: string;
	visits: number;
	unique_visitors: number;
}

// A dimension group keyed by sub-tab, e.g. environment.browser.
export type DashboardBreakdownGroup = Record<string, DashboardBreakdownItem[]>;

// The full personal dashboard model (GET /dashboard): every row reacts to the
// same from/to window.
export interface DashboardStats {
	total_links: number;
	active_links: number;
	disabled_links: number;
	lifetime_visits: number;
	window_visits: number;
	window_uniques: number;
	timeseries: TimeseriesRow[];
	top_by_visits: DashboardTopLink[];
	top_by_visitors: DashboardTopLink[];
	sources: DashboardBreakdownItem[];
	environment: DashboardBreakdownGroup;
	location: DashboardBreakdownGroup;
}

// An aggregate dimension breakdown across the caller's links (GET
// /stats/breakdowns) — the dashboard "More" dialog's data source.
export interface StatsBreakdown {
	dimension: string;
	total: number;
	items: DashboardBreakdownItem[];
	other: number;
}

export const DIMENSIONS = [
	'referrer',
	'device',
	'os',
	'browser',
	'country',
	'region',
	'city',
	'utm_source',
	'utm_medium',
	'utm_campaign',
	'utm_term',
	'utm_content',
	'utm_id'
] as const;

export type Dimension = (typeof DIMENSIONS)[number];
