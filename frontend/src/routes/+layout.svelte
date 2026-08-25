<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Link2, LogOut } from '@lucide/svelte';

	let { children, data } = $props();

	const isLogin = $derived(page.url.pathname === '/login');

	async function logout() {
		await api.logout();
		goto('/login');
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if !isLogin}
	<header class="border-b bg-background">
		<div class="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
			<div class="flex items-center gap-6">
				<a href="/" class="flex items-center gap-1.5 text-lg font-semibold tracking-tight">
					<Link2 class="size-5" />
					shrl.io
				</a>
				<nav class="flex items-center gap-4">
					<a href="/" class="text-sm text-muted-foreground hover:text-foreground">Links</a>
					<a href="/teams" class="text-sm text-muted-foreground hover:text-foreground">Teams</a>
					{#if data?.user?.isAdmin}
						<a href="/settings" class="text-sm text-muted-foreground hover:text-foreground">Settings</a>
					{/if}
				</nav>
			</div>
			<div class="flex items-center gap-3">
				<span class="text-sm text-muted-foreground">{data?.user?.username}</span>
				<Button variant="outline" onclick={logout}>
					<LogOut class="size-4" />
					Log out
				</Button>
			</div>
		</div>
	</header>
{/if}

<main class="mx-auto max-w-6xl px-4 py-8">
	{@render children()}
</main>
