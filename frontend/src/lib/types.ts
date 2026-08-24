export interface Link {
	hostname: string;
	code: string;
	destination: string;
	disabled: boolean;
	created_by: number;
	created_at: string;
	updated_at: string;
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
