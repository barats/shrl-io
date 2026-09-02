<script lang="ts">
	import { Monitor, Moon, Sun } from '@lucide/svelte';
	import { cycleTheme, themeState } from '$lib/theme.svelte';

	// The icon shows the active theme; clicking cycles light -> dark ->
	// system (system follows the OS and disables while in that mode).
	const label = $derived(
		themeState.theme === 'system'
			? `System theme; switch to light mode`
			: themeState.theme === 'dark'
				? 'Dark theme; switch to system theme'
				: 'Light theme; switch to dark mode'
	);
</script>

<button
	type="button"
	onclick={cycleTheme}
	aria-label={label}
	title={label}
	class="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
>
	{#if themeState.theme === 'system'}
		<Monitor class="size-4" />
	{:else if themeState.theme === 'dark'}
		<Moon class="size-4" />
	{:else}
		<Sun class="size-4" />
	{/if}
</button>
