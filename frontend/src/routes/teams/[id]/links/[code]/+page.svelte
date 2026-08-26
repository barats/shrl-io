<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api, daysAgo } from '$lib/api';
	import type {
		AnalyticsResponse,
		BreakdownResponse,
		Dimension,
		Link,
		TeamDetail,
		TimeseriesRow,
		User
	} from '$lib/types';
	import { DIMENSIONS } from '$lib/types';
	import VisitsChart from '$lib/components/VisitsChart.svelte';
	import LinkQR from '$lib/components/LinkQR.svelte';
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
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Power, PowerOff, Save, Trash2, TriangleAlert } from '@lucide/svelte';

	const teamId = $derived(Number(page.params.id));
	const code = $derived(page.params.code as string);

	let link = $state<Link | null>(null);
	let team = $state<TeamDetail | null>(null);
	let me = $state<User | null>(null);
	let loading = $state(true);
	let error = $state('');

	const hostname = $derived(link?.hostname ?? '');

	let editDestination = $state('');
	let editRemark = $state('');
	let editForwardUTM = $state(false);
	let saving = $state(false);
	let saveError = $state('');
	let saved = $state(false);

	let analytics = $state<AnalyticsResponse | null>(null);
	let timeseries = $state<TimeseriesRow[]>([]);
	let breakdowns = $state<BreakdownResponse | null>(null);
	let dimension = $state<Dimension>('referrer');
	let analyticsLoading = $state(true);
	let analyticsError = $state('');

	// A Team Link is managed by its Creator or a Team Owner; other members read.
	const canManage = $derived(
		!!link && (link.created_by === me?.id || team?.members.find((m) => m.id === me?.id)?.role === 'owner')
	);

	onMount(async () => {
		try {
			const [l, t, u] = await Promise.all([
				api.getLink(code),
				api.getTeam(teamId),
				api.me()
			]);
			link = l;
			team = t;
			me = u;
			editDestination = l.destination;
			editRemark = l.remark ?? '';
			editForwardUTM = l.forward_utm;
		} catch (e) {
			error = (e as Error).message;
			loading = false;
			return;
		}
		loading = false;
		await loadAnalytics();
	});

	async function loadAnalytics() {
		analyticsLoading = true;
		analyticsError = '';
		try {
			const from = daysAgo(30);
			[analytics, timeseries, breakdowns] = await Promise.all([
				api.getAnalytics(code),
				api.getTimeseries(code, from),
				api.getBreakdowns(code, dimension)
			]);
		} catch (e) {
			analyticsError = (e as Error).message;
		} finally {
			analyticsLoading = false;
		}
	}

	async function onDimensionChange(value: string | undefined) {
		if (!value) return;
		dimension = value as Dimension;
		await loadAnalytics();
	}

	async function saveLink() {
		saving = true;
		saveError = '';
		saved = false;
		try {
			link = await api.updateLink(code, editDestination, editRemark, editForwardUTM);
			saved = true;
		} catch (e) {
			saveError = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	async function toggleDisabled() {
		if (!link) return;
		try {
			link = await api.setDisabled(code, !link.disabled);
		} catch (e) {
			error = (e as Error).message;
		}
	}

	async function remove() {
		if (!link) return;
		if (!window.confirm(`Delete ${link.hostname}/${code}? This cannot be undone.`)) return;
		try {
			await api.deleteLink(code);
			await goto(`/teams/${teamId}`);
		} catch (e) {
			error = (e as Error).message;
		}
	}
</script>

{#if loading}
	<div class="space-y-4">
		<Skeleton class="h-8 w-64" />
		<Skeleton class="h-40 w-full" />
	</div>
{:else if error}
	<Alert variant="destructive">
		<TriangleAlert class="size-4" />
		<AlertTitle>Failed to load Link</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{:else if link && team}
	<div class="flex flex-wrap items-center gap-3">
		<h1 class="text-2xl font-semibold tracking-tight">
			<a href={`/teams/${teamId}`} class="text-muted-foreground hover:text-foreground hover:underline">
				{team.name}
			</a>
			<span class="text-muted-foreground">/</span>
			{hostname}/{code}
		</h1>
		{#if link.disabled}
			<Badge variant="secondary">Disabled</Badge>
		{:else}
			<Badge>Active</Badge>
		{/if}
	</div>
	<p class="mt-1 text-sm text-muted-foreground">
		Created {link.created_at.slice(0, 10)} · Updated {link.updated_at.slice(0, 10)}
		{#if !canManage}
			· Read-only — managed by its Creator or a Team Owner
		{/if}
	</p>

	<div class="mt-6 grid gap-6 lg:grid-cols-2">
		<div class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle>Destination</CardTitle>
					<CardDescription>Where this Link redirects to.</CardDescription>
				</CardHeader>
				<CardContent>
					{#if saveError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertDescription>{saveError}</AlertDescription>
						</Alert>
					{/if}
					<form
						onsubmit={(e) => {
							e.preventDefault();
							saveLink();
						}}
						class="space-y-3"
					>
						<div class="space-y-2">
							<Label for="destination">Destination URL</Label>
							<div class="flex gap-2">
								<Input
									id="destination"
									bind:value={editDestination}
									class="flex-1"
									required
									disabled={!canManage}
								/>
								{#if canManage}
									<Button type="submit" disabled={saving}>
										<Save class="size-4" /> {saving ? 'Saving…' : 'Save'}
									</Button>
								{/if}
							</div>
						</div>
						<div class="space-y-2">
							<Label for="remark">Remark (optional)</Label>
							<Input
								id="remark"
								bind:value={editRemark}
								placeholder="What this Link is for"
								disabled={!canManage}
							/>
						</div>
						<div class="flex items-start gap-2">
							<Checkbox id="forward-utm" bind:checked={editForwardUTM} disabled={!canManage} class="mt-0.5" />
							<Label
								for="forward-utm"
								class="font-normal leading-snug text-muted-foreground"
							>
								Forward UTM parameters from the short URL to the Destination
							</Label>
						</div>
						{#if saved}
							<p class="text-sm text-green-600">Link saved.</p>
						{/if}
					</form>
				</CardContent>
			</Card>

			{#if canManage}
				<Card>
					<CardHeader>
						<CardTitle>Status</CardTitle>
						<CardDescription>
							{link.disabled
								? 'This Link returns 404 from the redirector.'
								: 'This Link redirects visitors to its Destination.'}
						</CardDescription>
					</CardHeader>
					<CardContent class="flex flex-wrap gap-2">
						<Button variant={link.disabled ? 'default' : 'secondary'} onclick={toggleDisabled}>
							{#if link.disabled}
								<Power class="size-4" /> Enable
							{:else}
								<PowerOff class="size-4" /> Disable
							{/if}
						</Button>
						<Button variant="destructive" onclick={remove}>
							<Trash2 class="size-4" /> Delete
						</Button>
					</CardContent>
				</Card>
			{/if}

			<LinkQR {hostname} {code} />
		</div>

		<div class="space-y-6">
			<div class="grid grid-cols-3 gap-4">
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Lifetime visits
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading ? '…' : (analytics?.lifetime.visits ?? 0)}
						</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Window visits
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading ? '…' : (analytics?.window.visits ?? 0)}
						</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Unique visitors
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading ? '…' : (analytics?.window.unique_visitors ?? 0)}
						</p>
					</CardContent>
				</Card>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Visits (last 30 days)</CardTitle>
				</CardHeader>
				<CardContent>
					{#if analyticsLoading}
						<Skeleton class="h-40 w-full" />
					{:else if analyticsError}
						<Alert variant="destructive">
							<TriangleAlert class="size-4" />
							<AlertDescription>{analyticsError}</AlertDescription>
						</Alert>
					{:else}
						<VisitsChart rows={timeseries} />
					{/if}
				</CardContent>
			</Card>

			<Card>
				<CardHeader class="flex-row items-center justify-between space-y-0">
					<CardTitle>Breakdowns</CardTitle>
					<div class="w-44">
						<Select type="single" bind:value={dimension} onValueChange={onDimensionChange}>
							<SelectTrigger class="w-full">
								<span data-slot="select-value">{dimension}</span>
							</SelectTrigger>
							<SelectContent>
								{#each DIMENSIONS as d (d)}
									<SelectItem value={d} label={d} />
								{/each}
							</SelectContent>
						</Select>
					</div>
				</CardHeader>
				<CardContent>
					{#if analyticsLoading}
						<Skeleton class="h-40 w-full" />
					{:else if analyticsError}
						<Alert variant="destructive">
							<TriangleAlert class="size-4" />
							<AlertDescription>{analyticsError}</AlertDescription>
						</Alert>
					{:else if breakdowns}
						{#if breakdowns.total === 0}
							<p class="py-4 text-center text-sm text-muted-foreground">
								No visits in this window yet.
							</p>
						{:else}
							<div class="space-y-3">
								{#each breakdowns.items as item (dimension + JSON.stringify(item))}
									{@const entry = Object.entries(item)[0]}
									{@const value = entry?.[0] ?? 'unknown'}
									{@const count = entry?.[1] ?? 0}
									{@const pct = Math.round((count / breakdowns.total) * 100)}
									<div>
										<div class="flex items-center justify-between text-sm">
											<span class="truncate font-medium">{value}</span>
											<span class="text-muted-foreground">
												{count} · {pct}%
											</span>
										</div>
										<div class="mt-1 h-1.5 w-full rounded-full bg-muted">
											<div class="h-1.5 rounded-full bg-primary" style="width: {pct}%"></div>
										</div>
									</div>
								{/each}
								{#if breakdowns.other > 0}
									{@const pct = Math.round((breakdowns.other / breakdowns.total) * 100)}
									<div>
										<div class="flex items-center justify-between text-sm">
											<span class="truncate text-muted-foreground">Other</span>
											<span class="text-muted-foreground">
												{breakdowns.other} · {pct}%
											</span>
										</div>
										<div class="mt-1 h-1.5 w-full rounded-full bg-muted">
											<div class="h-1.5 rounded-full bg-muted-foreground/40" style="width: {pct}%"></div>
										</div>
									</div>
								{/if}
							</div>
						{/if}
					{/if}
					<Separator class="my-4" />
					<p class="text-xs text-muted-foreground">
						Bots and link-preview unfurlers are excluded. Rollups are pruned after the
						retention window; the lifetime total is never pruned.
					</p>
				</CardContent>
			</Card>
		</div>
	</div>
{/if}
