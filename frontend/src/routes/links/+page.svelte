<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { Link } from '$lib/types';
	import CreateLinkDialog from '$lib/components/CreateLinkDialog.svelte';
	import LinkList from '$lib/components/LinkList.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Plus } from '@lucide/svelte';

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
			links = await api.listLinks();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function loadLinks() {
		try {
			links = await api.listLinks();
		} catch (e) {
			error = (e as Error).message;
		}
	}
</script>

<svelte:head>
	<title>Links - shrl.io</title>
</svelte:head>

<div class="flex flex-wrap items-center justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">Links</h1>
	<Button onclick={() => (createOpen = true)}>
		<Plus class="size-4" /> Create Link
	</Button>
</div>

<LinkList {links} {loading} {error} hrefPrefix="/links" />

<CreateLinkDialog
	bind:open={createOpen}
	{baseURLs}
	{defaultBaseURL}
	onCreated={loadLinks}
/>
