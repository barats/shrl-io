import { daysAgo } from '$lib/api';
import type { TimeseriesRow } from '$lib/types';

export type RangePreset = 'today' | '7d' | '14d' | '30d' | '6mo' | '12mo' | 'all' | 'custom';

export const RANGE_PRESETS: { value: RangePreset; label: string }[] = [
	{ value: 'today', label: 'Today' },
	{ value: '7d', label: 'Last 7 days' },
	{ value: '14d', label: 'Last 14 days' },
	{ value: '30d', label: 'Last 30 days' },
	{ value: '6mo', label: 'Last 6 months' },
	{ value: '12mo', label: 'Last 12 months' },
	{ value: 'all', label: 'All time' }
];

// A resolved date window in UTC day granularity (the backend's only unit).
export interface DateRange {
	preset: RangePreset;
	from: string; // YYYY-MM-DD, inclusive
	to: string; // YYYY-MM-DD, inclusive
}

export function isRangePreset(v: string): v is RangePreset {
	return RANGE_PRESETS.some((p) => p.value === v) || v === 'custom';
}

function today(): string {
	return daysAgo(0);
}

export function rangeForPreset(preset: RangePreset): DateRange {
	switch (preset) {
		case 'today':
			return { preset, from: today(), to: today() };
		case '7d':
			return { preset, from: daysAgo(6), to: today() };
		case '14d':
			return { preset, from: daysAgo(13), to: today() };
		case '30d':
			return { preset, from: daysAgo(29), to: today() };
		case '6mo':
			return { preset, from: daysAgo(180), to: today() };
		case '12mo':
			return { preset, from: daysAgo(365), to: today() };
		case 'all':
			return { preset, from: '1970-01-01', to: today() };
		case 'custom':
			return { preset, from: today(), to: today() };
	}
}

export function presetLabel(preset: RangePreset): string {
	if (preset === 'custom') return 'Custom range';
	return RANGE_PRESETS.find((p) => p.value === preset)?.label ?? 'Custom range';
}

// bucketTimeseries collapses daily rows so long ranges stay readable: daily up
// to 90 days, weekly up to a year, monthly beyond. Buckets keep the chart to
// roughly 30–60 points.
export function bucketTimeseries(rows: TimeseriesRow[], from: string, to: string): TimeseriesRow[] {
	if (rows.length === 0) return rows;
	const spanDays = Math.max(1, Math.round((Date.parse(to) - Date.parse(from)) / 86_400_000));
	const bucket = spanDays <= 90 ? 'day' : spanDays <= 365 ? 'week' : 'month';
	if (bucket === 'day') return rows;

	const out: TimeseriesRow[] = [];
	let current: TimeseriesRow | null = null;
	for (const r of rows) {
		const key = bucket === 'week' ? weekStart(r.day) : monthStart(r.day);
		if (!current || current.day !== key) {
			current = { code: '', day: key, visits: 0, unique_visitors: 0 };
			out.push(current);
		}
		current.visits += r.visits;
		current.unique_visitors += r.unique_visitors;
	}
	return out;
}

// weekStart returns the Monday of the week containing day, as YYYY-MM-DD.
// The API returns days as RFC3339 ("2026-07-19T00:00:00Z"), so only the date
// part is parsed.
function weekStart(day: string): string {
	const datePart = day.slice(0, 10); // YYYY-MM-DD
	const d = new Date(datePart + 'T00:00:00Z');
	const offset = (d.getUTCDay() + 6) % 7; // Monday = 0
	d.setUTCDate(d.getUTCDate() - offset);
	return d.toISOString().slice(0, 10);
}

function monthStart(day: string): string {
	return day.slice(0, 7) + '-01';
}
