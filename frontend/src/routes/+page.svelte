<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import BreakdownDialog, { type BreakdownSection } from '$lib/components/BreakdownDialog.svelte';
	import RangeSelect from '$lib/components/RangeSelect.svelte';
	import RankCard from '$lib/components/RankCard.svelte';
	import StatsChart from '$lib/components/StatsChart.svelte';
	import WorldMap from '$lib/components/WorldMap.svelte';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { countryLabel } from '$lib/countries';
	import {
		bucketTimeseries,
		isRangePreset,
		presetLabel,
		rangeForPreset,
		type DateRange,
		type RangePreset
	} from '$lib/dashboard';
	import type { DashboardBreakdownItem, DashboardStats, DashboardTopLink } from '$lib/types';
	import { Link2, Plus, TriangleAlert } from '@lucide/svelte';

	const RANGE_KEY = 'shrl:range:preset';

	let data = $state<DashboardStats | null>(null);
	let range = $state<DateRange>(rangeForPreset('7d'));
	let loading = $state(true);
	let error = $state('');
	let envTab = $state('browser');
	let locTab = $state('country');
	let topTab = $state<'visits' | 'visitors'>('visits');
	let dialogOpen = $state(false);
	let dialogInitial = $state<string>('links');

	// The URL query is the single source of truth for the range; the effect
	// rebuilds the range and refetches on mount, on selection, and on
	// back/forward navigation.
	$effect(() => {
		const sp = page.url.searchParams;
		const preset = sp.get('preset');
		const from = sp.get('from');
		const to = sp.get('to');
		let next: DateRange;
		if (from && to) {
			next = { preset: 'custom', from, to };
		} else if (preset && isRangePreset(preset)) {
			next = rangeForPreset(preset as RangePreset);
		} else {
			const saved = localStorage.getItem(RANGE_KEY);
			const p = saved && isRangePreset(saved) ? (saved as RangePreset) : '7d';
			next = rangeForPreset(p);
		}
		range = next;
		load(next);
	});

	async function load(r: DateRange) {
		error = '';
		try {
			data = await api.getDashboard(r.from, r.to);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	function applyRange(r: DateRange) {
		const url = new URL(page.url);
		if (r.preset === 'custom') {
			url.searchParams.set('from', r.from);
			url.searchParams.set('to', r.to);
			url.searchParams.delete('preset');
		} else {
			url.searchParams.set('preset', r.preset);
			url.searchParams.delete('from');
			url.searchParams.delete('to');
			localStorage.setItem(RANGE_KEY, r.preset);
		}
		goto(url.pathname + url.search, { keepFocus: true });
	}

	const visitsCard = $derived(
		range.preset === 'all' ? (data?.lifetime_visits ?? 0) : (data?.window_visits ?? 0)
	);

	function topItems(links: DashboardTopLink[]): DashboardBreakdownItem[] {
		return links.map((l) => ({
			value: `${l.hostname}/${l.code}`,
			visits: l.visits,
			unique_visitors: l.unique_visitors
		}));
	}

	function topHrefs(links: DashboardTopLink[]): Record<string, string> {
		const out: Record<string, string> = {};
		for (const l of links) {
			out[`${l.hostname}/${l.code}`] = `/links/${encodeURIComponent(l.code)}`;
		}
		return out;
	}

	// The combined breakdown dialog's nav, config-driven so future dimensions
	// (e.g. utm_*) can be added without touching the dialog itself. The
	// fetchers close over the live `range`, so a dialog opened after a range
	// change reads the current window.
	const breakdownSections: BreakdownSection[] = [
		{
			id: 'links',
			label: 'Top Links',
			children: [
				{ id: 'visits', label: 'Visits' },
				{ id: 'visitors', label: 'Visitors' }
			],
			metric: (sub) => (sub === 'visits' ? 'visits' : 'visitors'),
			fetcher: async (sub) => {
				const m = sub === 'visits' ? 'visits' : 'visitors';
				const links = await api.getTopLinks(range.from, range.to, m, 0);
				return { items: topItems(links), hrefs: topHrefs(links) };
			}
		},
		{
			id: 'sources',
			label: 'Sources',
			fetcher: async () => ({
				items: (await api.getStatsBreakdowns('referrer', range.from, range.to, 0)).items
			})
		},
		{
			id: 'environment',
			label: 'Environment',
			children: [
				{ id: 'browser', label: 'Browser' },
				{ id: 'os', label: 'OS' },
				{ id: 'device', label: 'Device' }
			],
			fetcher: async (sub) => ({
				items: (await api.getStatsBreakdowns(sub ?? 'browser', range.from, range.to, 0)).items
			})
		},
		{
			id: 'location',
			label: 'Location',
			children: [
				{ id: 'country', label: 'Country' },
				{ id: 'region', label: 'Region' },
				{ id: 'city', label: 'City' }
			],
			valueFormatter: (v, sub) => (sub === 'country' ? countryLabel(v) : v),
			fetcher: async (sub) => ({
				items: (await api.getStatsBreakdowns(sub ?? 'country', range.from, range.to, 0)).items
			})
		}
	];

	const dialogConfig = $derived(
		dialogOpen ? { sections: breakdownSections, initial: dialogInitial } : null
	);

	function openDialog(id: string) {
		dialogInitial = id;
		dialogOpen = true;
	}
</script>

<svelte:head>
	<title>Dashboard — shrl.io</title>
</svelte:head>

<div class="flex flex-wrap items-center justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>
	<RangeSelect value={range} onchange={applyRange} />
</div>

{#if error}
	<Alert variant="destructive" class="mt-4">
		<TriangleAlert class="size-4" />
		<AlertTitle>Failed to load dashboard</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{/if}

<div class="mt-4 space-y-6">
	{#if loading && !data}
		<div class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-5">
			{#each [0, 1, 2, 3, 4] as i (i)}
				<Skeleton class="h-24 w-full" />
			{/each}
		</div>
		<Skeleton class="h-48 w-full" />
		<div class="grid gap-6 lg:grid-cols-2">
			<Skeleton class="h-64 w-full" />
			<Skeleton class="h-64 w-full" />
		</div>
		<div class="grid gap-6 lg:grid-cols-2">
			<Skeleton class="h-64 w-full" />
			<Skeleton class="h-64 w-full" />
		</div>
	{:else if data && data.total_links === 0}
		<Card class="mx-auto mt-10 max-w-md">
			<CardContent class="flex flex-col items-center gap-3 py-10 text-center">
				<Link2 class="size-8 text-muted-foreground" />
				<h2 class="text-lg font-semibold">Welcome to shrl.io</h2>
				<p class="text-sm text-muted-foreground">
					Create your first link to start seeing analytics here.
				</p>
				<Button href="/links">
					<Plus class="size-4" /> Create Link
				</Button>
			</CardContent>
		</Card>
	{:else if data}
		<!-- Row 1: stats cards. Total/Active/Disabled are current state and
		     ignore the range; Visits/Visitors are range-scoped. -->
		<div class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-5">
			<Card>
				<CardHeader class="pb-2">
					<CardTitle class="text-sm font-medium text-muted-foreground">Total Links</CardTitle>
				</CardHeader>
				<CardContent class="pt-0">
					<p class="text-2xl font-semibold">{data.total_links}</p>
				</CardContent>
			</Card>
			<Card>
				<CardHeader class="pb-2">
					<CardTitle class="text-sm font-medium text-muted-foreground">Active</CardTitle>
				</CardHeader>
				<CardContent class="pt-0">
					<p class="text-2xl font-semibold">{data.active_links}</p>
				</CardContent>
			</Card>
			<Card>
				<CardHeader class="pb-2">
					<CardTitle class="text-sm font-medium text-muted-foreground">Disabled</CardTitle>
				</CardHeader>
				<CardContent class="pt-0">
					<p class="text-2xl font-semibold">{data.disabled_links}</p>
				</CardContent>
			</Card>
			<Card>
				<CardHeader class="pb-2">
					<CardTitle class="text-sm font-medium text-muted-foreground">Visits</CardTitle>
				</CardHeader>
				<CardContent class="pt-0">
					<p class="text-2xl font-semibold">{visitsCard}</p>
					<p class="text-xs text-muted-foreground">{presetLabel(range.preset)}</p>
				</CardContent>
			</Card>
			<Card>
				<CardHeader class="pb-2">
					<CardTitle class="text-sm font-medium text-muted-foreground">Visitors</CardTitle>
				</CardHeader>
				<CardContent class="pt-0">
					<p class="text-2xl font-semibold">{data.window_uniques}</p>
					<p class="text-xs text-muted-foreground">{presetLabel(range.preset)}</p>
				</CardContent>
			</Card>
		</div>

		<!-- Row 2: visits & visitors chart -->
		<Card>
			<CardHeader>
				<CardTitle>Visits & Visitors ({presetLabel(range.preset)})</CardTitle>
			</CardHeader>
			<CardContent>
				<StatsChart rows={bucketTimeseries(data.timeseries, range.from, range.to)} />
			</CardContent>
		</Card>

		<!-- Row 3: top links + sources -->
		<div class="grid gap-6 lg:grid-cols-2">
			<RankCard
				title="Top Links"
				items={topItems(topTab === 'visits' ? data.top_by_visits : data.top_by_visitors)}
				hrefs={topHrefs(topTab === 'visits' ? data.top_by_visits : data.top_by_visitors)}
				tabs={['visits', 'visitors']}
				active={topTab}
				onTabChange={(t) => (topTab = t as 'visits' | 'visitors')}
				metric={topTab}
				onMore={() => openDialog('links')}
			/>
			<RankCard
				title="Sources"
				items={data.sources ?? []}
				metric="visitors"
				onMore={() => openDialog('sources')}
			/>
		</div>

		<!-- Row 4: environment + location -->
		<div class="grid gap-6 lg:grid-cols-2">
			<RankCard
				title="Environment"
				items={data.environment[envTab] ?? []}
				tabs={['browser', 'os', 'device']}
				active={envTab}
				onTabChange={(t) => (envTab = t)}
				metric="visitors"
				onMore={() => openDialog('environment')}
			/>
			<RankCard
				title="Location"
				items={data.location[locTab] ?? []}
				tabs={['country', 'region', 'city']}
				active={locTab}
				onTabChange={(t) => (locTab = t)}
				metric="visitors"
				valueFormatter={(v) => (locTab === 'country' ? countryLabel(v) : v)}
				onMore={() => openDialog('location')}
			/>
		</div>

		<!-- Row 5: world map (full width) -->
		<WorldMap from={range.from} to={range.to} />
	{/if}
</div>

<BreakdownDialog
	open={dialogOpen}
	config={dialogConfig}
	onclose={() => (dialogOpen = false)}
/>
