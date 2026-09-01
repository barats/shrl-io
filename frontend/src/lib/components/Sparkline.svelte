<script lang="ts">
	import type { TimeseriesRow } from '$lib/types';

	let { rows, class: className = '' }: { rows: TimeseriesRow[]; class?: string } = $props();

	// A tiny inline area/line mark of the visit series, normalized to a
	// 100x24 viewBox so it scales with the card width. Decorative; the
	// exact numbers live in the cards next to it.
	const points = $derived.by(() => {
		const values = rows.map((r) => r.visits);
		const max = Math.max(1, ...values);
		const min = Math.min(...values);
		const span = max - min || 1;
		const step = rows.length > 1 ? 100 / (rows.length - 1) : 0;
		return values
			.map((v, i) => {
				const x = rows.length > 1 ? i * step : 0;
				const y = 22 - ((v - min) / span) * 20;
				return `${x.toFixed(1)},${y.toFixed(1)}`;
			})
			.join(' ');
	});
</script>

<svg viewBox="0 0 100 24" preserveAspectRatio="none" class={className} aria-hidden="true">
	<polyline
		points={points}
		fill="none"
		stroke="currentColor"
		stroke-width="1.5"
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
</svg>
