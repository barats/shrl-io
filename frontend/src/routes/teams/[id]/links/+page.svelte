<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import type { Link } from '$lib/types';
	import CreateLinkDialog from '$lib/components/CreateLinkDialog.svelte';
	import LinkList from '$lib/components/LinkList.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Plus } from '@lucide/svelte';

	const teamId = $derived(page.params.id ?? '');
	const myRole = $derived(page.data.myRole);
	// Per the domain rules an Admin holds no implicit role in a Team, so
	// only actual members (myRole is null for outsider admins) can create
	// Links here.
	const canCreate = $derived(myRole !== null);

	let baseURLs = $state<string[]>([]);
	let defaultBaseURL = $state('');
	let links = $state<Link[]>([]);
	let loading = $state(true);
	let error = $state('');
	let createOpen = $state(false);

	onMount(async () => {
		try {
			const cfg = await api.config();
			const hs = await api.baseURLs();
			baseURLs = [...new Set([cfg.defaultBaseURL, ...hs])].sort();
			defaultBaseURL = cfg.defaultBaseURL;
			links = await api.listTeamLinks(teamId);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function loadLinks() {
		try {
			links = await api.listTeamLinks(teamId);
		} catch (e) {
			error = (e as Error).message;
		}
	}
</script>

<svelte:head>
	<title>Links - {page.data.team?.name ?? 'Team'} - shrl.io</title>
</svelte:head>

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Links</h1>
		{#if !canCreate}
			<p class="mt-1 text-sm text-muted-foreground">
				You are viewing this Team as an Admin. Only Team members can create Links here.
			</p>
		{/if}
	</div>
	{#if canCreate}
		<Button onclick={() => (createOpen = true)}>
			<Plus class="size-4" /> Create Link
		</Button>
	{/if}
</div>

<LinkList
	{links}
	{loading}
	{error}
	hrefPrefix={`/teams/${teamId}/links`}
	emptyHint={canCreate
		? undefined
		: 'Only Team members can create Links in this Team.'}
/>

<CreateLinkDialog
	bind:open={createOpen}
	{baseURLs}
	{defaultBaseURL}
	{teamId}
	onCreated={loadLinks}
/>
