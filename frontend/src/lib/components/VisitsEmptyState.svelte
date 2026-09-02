<script lang="ts">
	import { Link2 } from '@lucide/svelte';

	// Composed empty state for the Visits & Visitors chart when the selected
	// period has no visits. `link` scopes the hint to a single Link (detail
	// page); `overview` covers the dashboard and team views. `compact` drops
	// the hint for the small rank-card and chart fallbacks.
	let {
		scope = 'overview',
		compact = false
	}: { scope?: 'link' | 'overview'; compact?: boolean } = $props();
</script>

<div class="flex flex-col items-center gap-2 text-center {compact ? 'py-6' : 'py-10'}">
	<Link2 class="size-5 text-muted-foreground" />
	<p class="text-sm font-medium">No visits in this period</p>
	{#if !compact}
		<p class="max-w-xs text-xs text-muted-foreground">
			{#if scope === 'link'}
				Share this link to start collecting visits. Bots and link-preview unfurlers
				are excluded.
			{:else}
				Create and share Links to start collecting visits. Bots and link-preview
				unfurlers are excluded.
			{/if}
		</p>
	{/if}
</div>
