<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { ChevronDown, LogOut, Settings, UserRound, Users } from '@lucide/svelte';

	// A quick-switch menu between the caller's Personal context and each Team
	// they have joined, plus account links (Profile, admin-only Settings) and
	// Log out. The trigger shows the username; opening it lists Personal plus
	// the joined Teams (the current one marked) and Log out.
	let {
		username,
		teams,
		isAdmin = false,
		currentTeamId = null
	}: {
		username: string;
		teams: { id: string; name: string; role: string }[];
		isAdmin?: boolean;
		currentTeamId?: string | null;
	} = $props();

	let open = $state(false);
	let rootEl = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (!open) return;
		function onOutside(e: MouseEvent) {
			if (rootEl && !rootEl.contains(e.target as Node)) open = false;
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

	async function logout() {
		await api.logout();
		goto('/login');
	}

	function go(path: string) {
		open = false;
		goto(path);
	}
</script>

<div class="relative" bind:this={rootEl}>
	<button
		type="button"
		class="flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
		aria-haspopup="menu"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<UserRound class="size-4" />
		<span class="hidden sm:inline">{username}</span>
		<ChevronDown class="size-3.5 opacity-60" />
	</button>

	{#if open}
		<div
			class="absolute right-0 z-50 mt-1 w-60 rounded-md border bg-card p-1 text-sm shadow-md"
			role="menu"
		>
			<div class="px-2 py-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
				Switch to
			</div>
			<button
				type="button"
				class="flex w-full items-center justify-between gap-2 rounded px-2 py-1.5 text-left hover:bg-muted"
				role="menuitem"
				onclick={() => go('/')}
			>
				<span class="flex items-center gap-2">
					<UserRound class="size-4 text-muted-foreground" />
					Personal
				</span>
				{#if currentTeamId === null}
					<span class="text-xs text-muted-foreground">current</span>
				{/if}
			</button>

			{#if teams.length > 0}
				<div class="my-1 h-px bg-border"></div>
				<div class="px-2 py-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
					Teams
				</div>
				{#each teams as team (team.id)}
					<button
						type="button"
						class="flex w-full items-center justify-between gap-2 rounded px-2 py-1.5 text-left hover:bg-muted"
						role="menuitem"
						onclick={() => go(`/teams/${team.id}`)}
					>
						<span class="flex min-w-0 items-center gap-2">
							<Users class="size-4 shrink-0 text-muted-foreground" />
							<span class="truncate">{team.name}</span>
						</span>
						{#if currentTeamId === team.id}
							<span class="text-xs text-muted-foreground">current</span>
						{/if}
					</button>
				{/each}
			{/if}

			<div class="my-1 h-px bg-border"></div>
			<button
				type="button"
				class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left hover:bg-muted"
				role="menuitem"
				onclick={() => go('/profile')}
			>
				<UserRound class="size-4 text-muted-foreground" />
				Profile
			</button>
			{#if isAdmin}
				<button
					type="button"
					class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left hover:bg-muted"
					role="menuitem"
					onclick={() => go('/settings')}
				>
					<Settings class="size-4 text-muted-foreground" />
					Settings
				</button>
			{/if}

			<div class="my-1 h-px bg-border"></div>
			<button
				type="button"
				class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-red-600 hover:bg-muted"
				role="menuitem"
				onclick={logout}
			>
				<LogOut class="size-4" />
				Log out
			</button>
		</div>
	{/if}
</div>
