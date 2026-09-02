<script lang="ts">
	import { Menu, X } from '@lucide/svelte';

	// The mobile nav: a hamburger button that opens the same items in a menu,
	// shown only below sm. The layout places this where it belongs in mobile
	// order (the team layout shows it before the team name).
	let {
		items
	}: {
		items: { href: string; label: string; active?: boolean }[];
	} = $props();

	let open = $state(false);
	let rootEl = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (!open) return;
		// composedPath, not contains: the open toggle swaps the trigger's Menu/X
		// icons, detaching the clicked icon node mid-dispatch, which breaks contains.
		function onOutside(e: MouseEvent) {
			if (rootEl && !e.composedPath().includes(rootEl)) open = false;
		}
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape') open = false;
		}
		document.addEventListener('click', onOutside);
		document.addEventListener('keydown', onKey);
		return () => {
			document.removeEventListener('click', onOutside);
			document.removeEventListener('keydown', onKey);
		};
	});
</script>

<div class="relative sm:hidden" bind:this={rootEl}>
	<button
		type="button"
		class="flex items-center rounded-md px-2 py-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
		aria-haspopup="menu"
		aria-expanded={open}
		aria-label="Navigation"
		onclick={() => (open = !open)}
	>
		{#if open}
			<X class="size-5" />
		{:else}
			<Menu class="size-5" />
		{/if}
	</button>

	{#if open}
		<div
			class="absolute left-0 z-50 mt-1 w-52 rounded-md border bg-card p-1 text-sm shadow-md"
			role="menu"
		>
			{#each items as item (item.href)}
				<a
					href={item.href}
					class="block rounded px-3 py-2 {item.active
						? 'bg-muted font-medium text-foreground'
						: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
					role="menuitem"
				>
					{item.label}
				</a>
			{/each}
		</div>
	{/if}
</div>
