<script lang="ts">
	// The desktop/tablet nav: the items render inline on sm and up; hidden on
	// mobile (the MobileNav hamburger takes over there).
	let {
		items
	}: {
		items: { href: string; label: string; active?: boolean }[];
	} = $props();
</script>

<nav class="hidden items-center gap-4 sm:flex">
	{#each items as item (item.href)}
		<a
			href={item.href}
			class="text-sm {item.active
				? 'font-medium text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
		>
			{item.label}
		</a>
	{/each}
</nav>
