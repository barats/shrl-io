<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import type { Link } from '$lib/types';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select';
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

	let hostnames = $state<string[]>([]);
	let selectedHostname = $state('');
	let links = $state<Link[]>([]);
	let loading = $state(true);
	let error = $state('');

	let createHostname = $state('');
	let createDestination = $state('');
	let createRemark = $state('');
	let creating = $state(false);
	let createError = $state('');

	onMount(async () => {
		try {
			const cfg = await api.config();
			const initial = page.url.searchParams.get('hostname') || cfg.defaultHostname;
			const hs = await api.hostnames();
			hostnames = [...new Set([initial, cfg.defaultHostname, ...hs])].sort();
			selectedHostname = initial;
			createHostname = initial;
			await loadLinks();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function loadLinks() {
		loading = true;
		error = '';
		try {
			links = await api.listLinks(selectedHostname);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function onHostnameChange(value: string | undefined) {
		if (value === undefined) return;
		selectedHostname = value;
		createHostname = value;
		await loadLinks();
	}

	async function create() {
		creating = true;
		createError = '';
		try {
			await api.createLink({
				hostname: createHostname || undefined,
				destination: createDestination,
				remark: createRemark || undefined
			});
			const createdHost = createHostname;
			createDestination = '';
			createRemark = '';
			if (!hostnames.includes(createdHost)) {
				hostnames = [...hostnames, createdHost].sort();
			}
			selectedHostname = createdHost;
			await loadLinks();
		} catch (e) {
			createError = (e as Error).message;
		} finally {
			creating = false;
		}
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">Links</h1>

<div class="mt-4 grid gap-6 lg:grid-cols-3">
	<div class="lg:col-span-2">
		<Card>
			<CardHeader class="flex-row items-center justify-between space-y-0">
				<CardTitle>All Links</CardTitle>
				<div class="w-56">
					{#if loading && hostnames.length === 0}
						<Skeleton class="h-8 w-full" />
					{:else}
						<Select type="single" bind:value={selectedHostname} onValueChange={onHostnameChange}>
							<SelectTrigger class="w-full">
								<span data-slot="select-value">{selectedHostname || 'Hostname'}</span>
							</SelectTrigger>
							<SelectContent>
								{#each hostnames as host (host)}
									<SelectItem value={host} label={host} />
								{/each}
							</SelectContent>
						</Select>
					{/if}
				</div>
			</CardHeader>
			<CardContent>
				{#if error}
					<Alert variant="destructive">
						<TriangleAlert class="size-4" />
						<AlertTitle>Failed to load Links</AlertTitle>
						<AlertDescription>{error}</AlertDescription>
					</Alert>
				{:else if loading}
					<div class="space-y-3">
						{#each [0, 1, 2, 3] as i (i)}
							<Skeleton class="h-10 w-full" />
						{/each}
					</div>
				{:else if links.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">
						No Links on <span class="font-medium">{selectedHostname}</span> yet. Create one on the right.
					</p>
				{:else}
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Code</TableHead>
								<TableHead>Destination</TableHead>
								<TableHead class="w-24">Status</TableHead>
								<TableHead class="w-36">Created</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{#each links as link (link.hostname + link.code)}
								<TableRow>
									<TableCell class="font-medium">
										<a
											href={`/links/${encodeURIComponent(link.code)}?hostname=${encodeURIComponent(link.hostname)}`}
											class="inline-flex items-center gap-1.5 text-primary hover:underline"
										>
											<Link2 class="size-3.5" />
											{link.code}
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

	<div>
		<Card>
			<CardHeader>
				<CardTitle>Create a Link</CardTitle>
				<CardDescription>Shorten a Destination under this instance's hostnames.</CardDescription>
			</CardHeader>
			<CardContent>
				{#if createError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not create Link</AlertTitle>
						<AlertDescription>{createError}</AlertDescription>
					</Alert>
				{/if}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						create();
					}}
					class="space-y-4"
				>
					<div class="space-y-2">
						<Label for="new-hostname">Hostname</Label>
						<Select type="single" bind:value={createHostname}>
							<SelectTrigger id="new-hostname" class="w-full">
								<span data-slot="select-value">{createHostname || 'Hostname'}</span>
							</SelectTrigger>
							<SelectContent>
								{#each hostnames as host (host)}
									<SelectItem value={host} label={host} />
								{/each}
							</SelectContent>
						</Select>
					</div>
					<div class="space-y-2">
						<Label for="new-destination">Destination</Label>
						<Input
							id="new-destination"
							bind:value={createDestination}
							placeholder="https://example.com"
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="new-remark">Remark (optional)</Label>
						<Input
							id="new-remark"
							bind:value={createRemark}
							placeholder="What this Link is for"
						/>
					</div>
					<Button type="submit" class="w-full" disabled={creating}>
						{#if creating}
							Creating…
						{:else}
							<Plus class="size-4" /> Create Link
						{/if}
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
</div>
