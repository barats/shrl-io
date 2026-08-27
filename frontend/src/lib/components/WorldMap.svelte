<script lang="ts">
	import { geoNaturalEarth1, geoPath } from 'd3-geo';
	import type { Feature, FeatureCollection, Geometry } from 'geojson';
	import { feature as topoFeature } from 'topojson-client';
	import worldTopo from 'world-atlas/countries-110m.json';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { api } from '$lib/api';
	import { ISO2_TO_CCN3 } from '$lib/world';
	import { TriangleAlert } from '@lucide/svelte';

	let { from, to }: { from: string; to: string } = $props();

	// world-atlas countries-110m is a known Topology; decode to GeoJSON once.
	// Paths are stable, so only the fill color reacts to the fetched data.
	const topo = worldTopo as unknown as { objects: { countries: unknown } };
	const world = topoFeature(
		topo as never,
		topo.objects.countries as never
	) as unknown as FeatureCollection;

	const path = geoPath(geoNaturalEarth1().fitSize([960, 480], { type: 'Sphere' }));
	// world-atlas includes unnumbered territories (no id); skip them so every
	// shape key is unique and mappable to the alpha-2 analytics data.
	const shapes = world.features
		.filter((f) => f.id != null && String(f.id) !== '')
		.map((f) => ({
			id: String(f.id ?? ''),
			name: ((f.properties as { name?: string } | null)?.name ?? String(f.id ?? '')) as string,
			d: path(f as unknown as Feature<Geometry>) ?? ''
		}));
	const byId = new Map(shapes.map((s) => [s.id, s]));

	function themeColor(name: string, alpha: number): string {
		if (typeof document === 'undefined') return `rgba(128, 128, 128, ${alpha})`;
		const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
		if (!raw) return `rgba(128, 128, 128, ${alpha})`;
		if (raw.endsWith(')')) {
			return `${raw.slice(0, -1)} / ${alpha})`;
		}
		return raw;
	}

	let counts = $state<Map<string, { visits: number; unique_visitors: number }>>(new Map());
	let maxVisitors = $state(0);
	let unknownVisits = $state(0);
	let loading = $state(true);
	let error = $state('');
	let hover = $state<{
		name: string;
		visits: number;
		visitors: number;
		x: number;
		y: number;
	} | null>(null);

	$effect(() => {
		loading = true;
		error = '';
		api
			.getStatsBreakdowns('country', from, to, 0)
			.then((b) => {
				const m = new Map<string, { visits: number; unique_visitors: number }>();
				let mx = 0;
				let unknown = 0;
				for (const it of b.items) {
					if (it.value === 'unknown') {
						unknown += it.visits;
						continue;
					}
					const ccn3 = ISO2_TO_CCN3[it.value];
					if (!ccn3) continue;
					m.set(ccn3, { visits: it.visits, unique_visitors: it.unique_visitors });
					mx = Math.max(mx, it.unique_visitors);
				}
				counts = m;
				maxVisitors = mx;
				unknownVisits = unknown;
			})
			.catch((e) => {
				error = (e as Error).message;
			})
			.finally(() => {
				loading = false;
			});
	});

	function fill(id: string): string {
		const c = counts.get(id);
		if (!c || c.unique_visitors <= 0 || maxVisitors <= 0) return themeColor('--muted', 0.55);
		const step = Math.min(4, Math.floor((c.unique_visitors / maxVisitors) * 5));
		return themeColor('--primary', 0.15 + step * 0.17);
	}
</script>

<Card class="mt-6 w-full">
	<CardHeader>
		<CardTitle>World Map</CardTitle>
		<CardDescription>Visitors by country</CardDescription>
	</CardHeader>
	<CardContent>
		{#if loading && counts.size === 0}
			<Skeleton class="h-72 w-full" />
		{:else if error}
			<Alert variant="destructive">
				<TriangleAlert class="size-4" />
				<AlertTitle>Failed to load map</AlertTitle>
				<AlertDescription>{error}</AlertDescription>
			</Alert>
		{:else}
			<div class="relative">
				<svg
					viewBox="0 0 960 480"
					class="h-auto w-full"
					role="img"
					aria-label="World map of visitor origins by country"
					onmousemove={(e) => {
						const id = (e.target as SVGElement | null)?.getAttribute('data-id');
						if (!id) return;
						const s = byId.get(id);
						if (!s) return;
						const c = counts.get(id);
						hover = {
							name: s.name,
							visits: c?.visits ?? 0,
							visitors: c?.unique_visitors ?? 0,
							x: e.clientX + 12,
							y: e.clientY + 12
						};
					}}
					onmouseleave={() => (hover = null)}
				>
					{#each shapes as s (s.id)}
						<path
							d={s.d}
							fill={fill(s.id)}
							data-id={s.id}
							class="cursor-pointer transition-opacity hover:opacity-80"
						></path>
					{/each}
				</svg>
				{#if hover}
					<div
						class="pointer-events-none fixed z-50 rounded-md border bg-card px-2 py-1 text-xs text-foreground shadow-md"
						style="left: {hover.x}px; top: {hover.y}px"
					>
						<span class="font-medium">{hover.name}</span>
						<span class="text-muted-foreground">
							· {hover.visitors} visitors · {hover.visits} visits
						</span>
					</div>
				{/if}
			</div>
			<div class="mt-4 flex items-center gap-3 text-xs text-muted-foreground">
				<span>Fewer</span>
				<div
					class="h-2 w-40 rounded-full"
					style="background: linear-gradient(to right, {themeColor('--primary', 0.15)}, {themeColor('--primary', 0.83)})"
				></div>
				<span>More</span>
				<span class="ml-2 inline-flex items-center gap-1.5">
					<span class="size-2.5 rounded-sm bg-muted"></span>
					No data
				</span>
			</div>
			{#if unknownVisits > 0}
				<p class="mt-2 text-xs text-muted-foreground">
					{unknownVisits} visits could not be attributed to a country.
				</p>
			{/if}
		{/if}
	</CardContent>
</Card>
