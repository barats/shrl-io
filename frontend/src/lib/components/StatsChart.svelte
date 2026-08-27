<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		BarController,
		BarElement,
		CategoryScale,
		Chart,
		Legend,
		LinearScale,
		Tooltip
	} from 'chart.js';
	import type { TimeseriesRow } from '$lib/types';

	Chart.register(BarController, BarElement, CategoryScale, LinearScale, Tooltip, Legend);

	let { rows }: { rows: TimeseriesRow[] } = $props();

	let canvas = $state<HTMLCanvasElement | null>(null);
	let chart: Chart | null = null;

	// themeColor resolves a design-token CSS variable (e.g. --primary) and
	// applies alpha, so the chart matches the app's light/dark palette.
	function themeColor(name: string, alpha: number): string {
		const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
		if (!raw) return `rgba(128, 128, 128, ${alpha})`;
		if (raw.endsWith(')')) {
			return `${raw.slice(0, -1)} / ${alpha})`;
		}
		return raw;
	}

	function shortDay(day: string): string {
		const [, m, d] = day.slice(0, 10).split('-');
		return `${Number(m)}/${Number(d)}`;
	}

	$effect(() => {
		if (!canvas) {
			chart?.destroy();
			chart = null;
			return;
		}
		if (!chart) {
			chart = new Chart(canvas, {
				type: 'bar',
				data: {
					labels: [],
					datasets: [
						{
							label: 'Visits',
							data: [],
							backgroundColor: themeColor('--primary', 0.85),
							borderRadius: 3
						},
						{
							label: 'Visitors',
							data: [],
							backgroundColor: themeColor('--muted-foreground', 0.45),
							borderRadius: 3
						}
					]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					interaction: { mode: 'index', intersect: false },
					plugins: {
						legend: { display: false },
						tooltip: {
							callbacks: {
								title(items) {
									const idx = items[0]?.dataIndex ?? 0;
									return rows[idx]?.day.slice(0, 10) ?? '';
								}
							}
						}
					},
					scales: {
						x: {
							grid: { display: false },
							ticks: {
								color: themeColor('--muted-foreground', 0.8),
								maxTicksLimit: 12,
								maxRotation: 0
							}
						},
						y: {
							beginAtZero: true,
							grid: { color: themeColor('--border', 0.5) },
							ticks: {
								color: themeColor('--muted-foreground', 0.8),
								precision: 0
							}
						}
					}
				}
			});
		}
		chart.data.labels = rows.map((r) => shortDay(r.day));
		chart.data.datasets[0].data = rows.map((r) => r.visits);
		chart.data.datasets[1].data = rows.map((r) => r.unique_visitors);
		chart.update();
	});

	onDestroy(() => {
		chart?.destroy();
		chart = null;
	});

	// Re-resolve the token colors when the OS color scheme flips, so an open
	// chart keeps matching the active palette (light/dark).
	onMount(() => {
		const scheme = window.matchMedia('(prefers-color-scheme: dark)');
		const onSchemeChange = () => {
			if (!chart) return;
			chart.data.datasets[0].backgroundColor = themeColor('--primary', 0.85);
			chart.data.datasets[1].backgroundColor = themeColor('--muted-foreground', 0.45);
			chart.update();
		};
		scheme.addEventListener('change', onSchemeChange);
		onDestroy(() => scheme.removeEventListener('change', onSchemeChange));
	});
</script>

{#if rows.length === 0}
	<p class="text-sm text-muted-foreground">No visits in this period yet.</p>
{:else}
	<div class="space-y-3">
		<div class="flex items-center gap-4 text-xs text-muted-foreground">
			<span class="inline-flex items-center gap-1.5">
				<span class="size-2.5 rounded-sm bg-primary opacity-85"></span>
				Visits
			</span>
			<span class="inline-flex items-center gap-1.5">
				<span class="size-2.5 rounded-sm bg-muted-foreground/45"></span>
				Visitors
			</span>
		</div>
		<div class="h-60 w-full">
			<canvas bind:this={canvas}></canvas>
		</div>
	</div>
{/if}
