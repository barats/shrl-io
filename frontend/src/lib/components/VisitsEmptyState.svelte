<script lang="ts">
	import { Link2 } from '@lucide/svelte';

	// Composed empty state for the Visits & Visitors chart when the selected
	// period has no visits. `link` scopes the hint to a single Link (detail
	// page); `overview` covers the dashboard and team views.
	let { scope = 'overview' }: { scope?: 'link' | 'overview' } = $props();
</script>

<div class="flex flex-col items-center gap-2 py-10 text-center">
	<Link2 class="size-6 text-muted-foreground" />
	<p class="text-sm font-medium">No visits in this period</p>
	<p class="max-w-xs text-xs text-muted-foreground">
		{#if scope === 'link'}
			Share this link to start collecting visits. Bots and link-preview unfurlers
			are excluded.
		{:else}
			Create and share Links to start collecting visits. Bots and link-preview
			unfurlers are excluded.
		{/if}
	</p>
</div>
