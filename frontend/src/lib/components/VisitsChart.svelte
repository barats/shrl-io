<script lang="ts">
	import type { TimeseriesRow } from '$lib/types';

	let { rows }: { rows: TimeseriesRow[] } = $props();

	const max = $derived(Math.max(1, ...rows.map((r) => r.visits)));
	const H = 160;
	const W = $derived(Math.max(320, rows.length * 8));
	const labelEvery = $derived(Math.max(1, Math.ceil(rows.length / 8)));
</script>

{#if rows.length === 0}
	<p class="text-sm text-muted-foreground">No visits in this period yet.</p>
{:else}
	<div class="w-full overflow-x-auto">
		<svg width={W} height={H + 24} class="block">
			{#each rows as row, i (row.day)}
				<rect
					x={i * 8}
					y={H - (row.visits / max) * H}
					width={6}
					height={(row.visits / max) * H}
					rx={1}
					fill="currentColor"
					class="text-primary opacity-80"
				>
					<title>{row.day}: {row.visits} visits</title>
				</rect>
			{/each}
			{#each rows as row, i (row.day)}
				{#if i % labelEvery === 0}
					<text
						x={i * 8 + 3}
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
{/if}
