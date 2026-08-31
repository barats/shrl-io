<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import InlineNav from '$lib/components/InlineNav.svelte';
	import MobileNav from '$lib/components/MobileNav.svelte';
	import UserMenu from '$lib/components/UserMenu.svelte';

	let { children, data } = $props();

	const isLogin = $derived(page.url.pathname === '/login');
	// A Team route (a numeric id under /teams) renders its own navbar from the
	// nested teams/[id] layout; everything else shows the personal navbar.
	const isTeamRoute = $derived(/^\/teams\/\d+(\/|$)/.test(page.url.pathname));

	const path = $derived(page.url.pathname);
	const nav = $derived([
		{ href: '/', label: 'Dashboard', active: path === '/' },
		{ href: '/links', label: 'Links', active: path.startsWith('/links') },
		{ href: '/teams', label: 'Teams', active: path.startsWith('/teams') },
		{ href: '/account', label: 'Account', active: path.startsWith('/account') },
		...(data?.user?.isAdmin
			? [{ href: '/settings', label: 'Settings', active: path.startsWith('/settings') }]
			: [])
	]);
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
					<a href="/" class="hidden items-center text-lg font-semibold tracking-tight sm:flex">
						shrl.io
					</a>
					<MobileNav items={nav} />
					<InlineNav items={nav} />
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
