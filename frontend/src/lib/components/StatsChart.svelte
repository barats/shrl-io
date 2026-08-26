<script lang="ts">
	import type { TimeseriesRow } from '$lib/types';

	let { rows }: { rows: TimeseriesRow[] } = $props();

	const max = $derived(Math.max(1, ...rows.flatMap((r) => [r.visits, r.unique_visitors])));
	const H = 160;
	const W = $derived(Math.max(320, rows.length * 14));
	const labelEvery = $derived(Math.max(1, Math.ceil(rows.length / 8)));
</script>

{#if rows.length === 0}
	<p class="text-sm text-muted-foreground">No visits in this period yet.</p>
{:else}
	<div class="space-y-3">
		<div class="flex items-center gap-4 text-xs text-muted-foreground">
			<span class="inline-flex items-center gap-1.5">
				<span class="size-2.5 rounded-sm bg-primary opacity-80"></span>
				Visits
			</span>
			<span class="inline-flex items-center gap-1.5">
				<span class="size-2.5 rounded-sm bg-muted-foreground/50"></span>
				Visitors
			</span>
		</div>
		<div class="w-full overflow-x-auto">
			<svg width={W} height={H + 24} class="block">
				{#each rows as row, i (row.day)}
					<rect
						x={i * 14}
						y={H - (row.visits / max) * H}
						width={6}
						height={(row.visits / max) * H}
						rx={1}
						fill="currentColor"
						class="text-primary opacity-80"
					>
						<title>{row.day}: {row.visits} visits</title>
					</rect>
					<rect
						x={i * 14 + 8}
						y={H - (row.unique_visitors / max) * H}
						width={6}
						height={(row.unique_visitors / max) * H}
						rx={1}
						fill="currentColor"
						class="text-muted-foreground/50"
					>
						<title>{row.day}: {row.unique_visitors} visitors</title>
					</rect>
				{/each}
				{#each rows as row, i (row.day)}
					{#if i % labelEvery === 0}
						<text
							x={i * 14 + 7}
							y={H + 16}
							text-anchor="middle"
							fill="currentColor"
							class="text-[10px] text-muted-foreground"
						>
							{row.day.slice(0, 10)}
						</text>
					{/if}
				{/each}
			</svg>
		</div>
	</div>
{/if}
