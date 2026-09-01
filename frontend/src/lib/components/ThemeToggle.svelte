<script lang="ts">
	import { Moon, Sun } from '@lucide/svelte';
	import { themeState, toggleTheme } from '$lib/theme.svelte';
</script>

<button
	type="button"
	onclick={toggleTheme}
	aria-label={themeState.effectiveDark ? 'Switch to light mode' : 'Switch to dark mode'}
	title={themeState.effectiveDark ? 'Switch to light mode' : 'Switch to dark mode'}
	class="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
>
	{#if themeState.effectiveDark}
		<Sun class="size-4" />
	{:else}
		<Moon class="size-4" />
	{/if}
</button>
