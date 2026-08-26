<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
</script>

<svelte:head>
	<title>{page.status === 404 ? 'Page not found — shrl.io' : 'Something went wrong — shrl.io'}</title>
</svelte:head>

<div class="flex min-h-[60vh] items-center justify-center">
	<div class="w-full max-w-sm text-center">
		<p class="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
			{page.status}
		</p>
		{#if page.status === 404}
			<h1 class="mt-2 text-2xl font-semibold tracking-tight">Page not found</h1>
			<p class="mt-2 text-sm text-muted-foreground">
				The page you're looking for doesn't exist.
			</p>
		{:else}
			<h1 class="mt-2 text-2xl font-semibold tracking-tight">Something went wrong</h1>
			<p class="mt-2 text-sm text-muted-foreground">
				{page.error?.message ?? 'An unexpected error occurred.'}
			</p>
		{/if}
	</div>
</div>
