<script lang="ts">
	import { page } from '$app/state';
	import InlineNav from '$lib/components/InlineNav.svelte';
	import MobileNav from '$lib/components/MobileNav.svelte';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import UserMenu from '$lib/components/UserMenu.svelte';

	let { children, data } = $props();

	const teamId = $derived(page.params.id ?? '');
	const team = $derived(data.team);
	const path = $derived(page.url.pathname);

	function isActive(href: string): boolean {
		if (href === `/teams/${teamId}`) return path === href;
		return path.startsWith(href + '/') || path === href;
	}

	const nav = $derived([
		{ href: `/teams/${teamId}`, label: 'Overview', active: isActive(`/teams/${teamId}`) },
		{ href: `/teams/${teamId}/links`, label: 'Links', active: isActive(`/teams/${teamId}/links`) },
		{ href: `/teams/${teamId}/settings`, label: 'Settings', active: isActive(`/teams/${teamId}/settings`) }
	]);
</script>

<svelte:head>
	<title>{team?.name ?? 'Team'} - shrl.io</title>
</svelte:head>

{#if team}
	<header class="border-b bg-background">
		<div class="mx-auto flex min-h-14 max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4">
			<div class="flex flex-wrap items-center gap-x-6 gap-y-2">
				<a href="/" class="hidden items-center text-lg font-semibold tracking-tight sm:flex">
					shrl.io
				</a>
				<MobileNav items={nav} />
				<span class="max-w-40 truncate font-medium text-foreground">{team.name}</span>
				<InlineNav items={nav} />
			</div>
			<div class="flex items-center gap-1">
				<ThemeToggle />
				<UserMenu
					username={data.user?.username ?? ''}
					teams={data.teams ?? []}
					isAdmin={data.user?.isAdmin ?? false}
					currentTeamId={teamId}
				/>
			</div>
		</div>
	</header>
{/if}

<main class="mx-auto max-w-6xl px-4 py-8">
	{@render children()}
</main>
