<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import UserMenu from '$lib/components/UserMenu.svelte';
	import { Link2 } from '@lucide/svelte';

	let { children, data } = $props();

	const isLogin = $derived(page.url.pathname === '/login');
	// A Team route (a numeric id under /teams) renders its own navbar from the
	// nested teams/[id] layout; everything else shows the personal navbar.
	const isTeamRoute = $derived(/^\/teams\/\d+(\/|$)/.test(page.url.pathname));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if isTeamRoute}
	<!-- the teams/[id] layout provides the team navbar + main wrapper -->
	{@render children()}
{:else}
	{#if !isLogin}
		<header class="border-b bg-background">
			<div class="mx-auto flex min-h-14 max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4">
				<div class="flex items-center gap-6">
					<a href="/" class="flex items-center gap-1.5 text-lg font-semibold tracking-tight">
						<Link2 class="size-5" />
						<span class="hidden sm:inline">shrl.io</span>
					</a>
					<nav class="flex items-center gap-4">
						<a href="/" class="text-sm text-muted-foreground hover:text-foreground">Dashboard</a>
						<a href="/links" class="text-sm text-muted-foreground hover:text-foreground">Links</a>
						<a href="/teams" class="text-sm text-muted-foreground hover:text-foreground">Teams</a>
						<a href="/account" class="text-sm text-muted-foreground hover:text-foreground">Account</a>
						{#if data?.user?.isAdmin}
							<a href="/settings" class="text-sm text-muted-foreground hover:text-foreground">Settings</a>
						{/if}
					</nav>
				</div>
				{#if data?.user}
					<UserMenu username={data.user.username} teams={data.teams ?? []} currentTeamId={null} />
				{/if}
			</div>
		</header>
	{/if}

	<main class="mx-auto max-w-6xl px-4 py-8">
		{@render children()}
	</main>
{/if}
