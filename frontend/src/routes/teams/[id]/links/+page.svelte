<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import type { Link } from '$lib/types';
	import CreateLinkDialog from '$lib/components/CreateLinkDialog.svelte';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import {
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TableRow
	} from '$lib/components/ui/table';
	import { Link2, Plus, TriangleAlert } from '@lucide/svelte';

	const teamId = $derived(Number(page.params.id));

	let hostnames = $state<string[]>([]);
	let defaultHostname = $state('');
	let links = $state<Link[]>([]);
	let loading = $state(true);
	let error = $state('');
	let createOpen = $state(false);

	onMount(async () => {
		try {
			const cfg = await api.config();
			const hs = await api.hostnames();
			hostnames = [...new Set([cfg.defaultHostname, ...hs])].sort();
			defaultHostname = cfg.defaultHostname;
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
	<title>Links — {page.data.team?.name ?? 'Team'} — shrl.io</title>
</svelte:head>

<div class="flex flex-wrap items-center justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">Links</h1>
	<Button onclick={() => (createOpen = true)}>
		<Plus class="size-4" /> Create Link
	</Button>
</div>

{#if error}
	<Alert variant="destructive" class="mt-4">
		<TriangleAlert class="size-4" />
		<AlertTitle>Failed to load Links</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{/if}

<div class="mt-4 space-y-6">
	<Card>
		<CardContent class="pt-6">
			{#if loading}
				<div class="space-y-3">
					{#each [0, 1, 2] as i (i)}
						<Skeleton class="h-10 w-full" />
					{/each}
				</div>
			{:else if links.length === 0}
				<p class="py-8 text-center text-sm text-muted-foreground">
					No Links yet. Create one with the button above.
				</p>
			{:else}
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Link</TableHead>
							<TableHead>Destination</TableHead>
							<TableHead class="w-24">Status</TableHead>
							<TableHead class="w-36">Created</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{#each links as link (link.code)}
							<TableRow>
								<TableCell class="font-medium">
									<a
										href={`/teams/${teamId}/links/${encodeURIComponent(link.code)}`}
										class="inline-flex items-center gap-1.5 text-primary hover:underline"
									>
										<Link2 class="size-3.5" />
										{link.hostname}/{link.code}
									</a>
								</TableCell>
								<TableCell class="max-w-72 truncate text-muted-foreground">
									{link.destination}
								</TableCell>
								<TableCell>
									{#if link.disabled}
										<Badge variant="secondary">Disabled</Badge>
									{:else}
										<Badge>Active</Badge>
									{/if}
								</TableCell>
								<TableCell class="text-muted-foreground">
									{link.created_at.slice(0, 10)}
								</TableCell>
							</TableRow>
						{/each}
					</TableBody>
				</Table>
			{/if}
		</CardContent>
	</Card>
</div>

<CreateLinkDialog
	bind:open={createOpen}
	{hostnames}
	{defaultHostname}
	{teamId}
	onCreated={loadLinks}
/>
