<script lang="ts">
	import { page } from '$app/state';
	import UserMenu from '$lib/components/UserMenu.svelte';
	import { ChevronRight, Link2 } from '@lucide/svelte';

	let { children, data } = $props();

	const teamId = $derived(Number(page.params.id));
	const team = $derived(data.team);
	const path = $derived(page.url.pathname);

	function isActive(href: string): boolean {
		if (href === `/teams/${teamId}`) return path === href;
		return path.startsWith(href + '/') || path === href;
	}

	const nav = $derived([
		{ href: `/teams/${teamId}`, label: 'Overview' },
		{ href: `/teams/${teamId}/links`, label: 'Links' },
		{ href: `/teams/${teamId}/members`, label: 'Members' },
		{ href: `/teams/${teamId}/settings`, label: 'Settings' }
	]);
</script>

<svelte:head>
	<title>{team?.name ?? 'Team'} — shrl.io</title>
</svelte:head>

{#if team}
	<header class="border-b bg-background">
		<div class="mx-auto flex min-h-14 max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4">
			<div class="flex flex-wrap items-center gap-x-6 gap-y-2">
				<a href="/" class="flex items-center gap-1.5 text-lg font-semibold tracking-tight">
					<Link2 class="size-5" />
					<span class="hidden sm:inline">shrl.io</span>
				</a>
				<div class="flex items-center gap-1.5 text-sm text-muted-foreground">
					<a href="/" class="hover:text-foreground">Personal</a>
					<ChevronRight class="size-3.5" />
					<span class="font-medium text-foreground">{team.name}</span>
				</div>
				<nav class="flex items-center gap-4">
					{#each nav as item (item.href)}
						<a
							href={item.href}
							class="text-sm {isActive(item.href)
								? 'font-medium text-foreground'
								: 'text-muted-foreground hover:text-foreground'}"
						>
							{item.label}
						</a>
					{/each}
				</nav>
			</div>
			<UserMenu
				username={data.user?.username ?? ''}
				teams={data.teams ?? []}
				currentTeamId={teamId}
			/>
		</div>
	</header>
{/if}

<main class="mx-auto max-w-6xl px-4 py-8">
	{@render children()}
</main>
