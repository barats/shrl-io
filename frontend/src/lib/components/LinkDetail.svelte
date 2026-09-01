<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import BreakdownDialog, {
		type BreakdownSection
	} from '$lib/components/BreakdownDialog.svelte';
	import LinkQR from '$lib/components/LinkQR.svelte';
	import RangeSelect from '$lib/components/RangeSelect.svelte';
	import RankCard from '$lib/components/RankCard.svelte';
	import StatsChart from '$lib/components/StatsChart.svelte';
	import WorldMap from '$lib/components/WorldMap.svelte';
	import { countryLabel } from '$lib/countries';
	import {
		bucketTimeseries,
		isRangePreset,
		presetLabel,
		rangeForPreset,
		type DateRange,
		type RangePreset
	} from '$lib/dashboard';
	import type {
		AnalyticsResponse,
		DashboardBreakdownItem,
		Link,
		TeamDetail,
		TimeseriesRow,
		User
	} from '$lib/types';
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
	import { Skeleton } from '$lib/components/ui/skeleton';
	import {
		ChevronLeft,
		Copy,
		ExternalLink,
		Link2,
		Power,
		PowerOff,
		Save,
		Trash2,
		TriangleAlert
	} from '@lucide/svelte';

	const RANGE_KEY = 'shrl:range:preset';

	let { code, teamId }: { code: string; teamId?: number } = $props();

	let link = $state<Link | null>(null);
	let team = $state<TeamDetail | null>(null);
	let me = $state<User | null>(null);
	let loading = $state(true);
	let error = $state('');

	let editDestination = $state('');
	let editRemark = $state('');
	let editForwardUTM = $state(false);
	let saving = $state(false);
	let saveError = $state('');
	let saved = $state(false);

	let analytics = $state<AnalyticsResponse | null>(null);
	let timeseries = $state<TimeseriesRow[]>([]);
	let analyticsLoading = $state(true);
	let analyticsError = $state('');

	let sourceTab = $state('referrer');
	let envTab = $state('browser');
	let locTab = $state('country');
	let campaignTab = $state('utm_source');
	let sourceItems = $state<DashboardBreakdownItem[]>([]);
	let envItems = $state<DashboardBreakdownItem[]>([]);
	let locItems = $state<DashboardBreakdownItem[]>([]);
	let campaignItems = $state<DashboardBreakdownItem[]>([]);

	let dialogOpen = $state(false);
	let dialogInitial = $state('sources');

	let copied = $state(false);

	// Per-card fetch guards so a slow response for a previous tab can't
	// overwrite the tab the user has since switched to.
	let srcSeq = 0;
	let envSeq = 0;
	let locSeq = 0;
	let campSeq = 0;

	const hostname = $derived(link?.hostname ?? '');
	const shortUrl = $derived(link ? `https://${hostname}/${code}` : '');
	const backHref = $derived(teamId ? `/teams/${teamId}` : '/links');
	const backLabel = $derived(team ? team.name : 'All links');
	const canManage = $derived(
		!!link &&
			(teamId
				? link.created_by === me?.id ||
					team?.members.find((m) => m.id === me?.id)?.role === 'owner'
				: true)
	);

	// The URL query is the single source of truth for the range; the effect
	// rebuilds the range and refetches analytics on mount, on selection, and
	// on back/forward navigation (same pattern as the dashboard).
	let range = $state<DateRange>(rangeForPreset('7d'));

	$effect(() => {
		const sp = page.url.searchParams;
		const preset = sp.get('preset');
		const from = sp.get('from');
		const to = sp.get('to');
		let next: DateRange;
		if (from && to) {
			next = { preset: 'custom', from, to };
		} else if (preset && isRangePreset(preset)) {
			next = rangeForPreset(preset as RangePreset);
		} else {
			const saved = localStorage.getItem(RANGE_KEY);
			const p = saved && isRangePreset(saved) ? (saved as RangePreset) : '7d';
			next = rangeForPreset(p);
		}
		range = next;
		loadAnalytics(next.from, next.to);
	});

	onMount(async () => {
		try {
			let l: Link;
			if (teamId) {
				const [ll, t, u] = await Promise.all([
					api.getLink(code),
					api.getTeam(teamId),
					api.me()
				]);
				l = ll;
				team = t;
				me = u;
			} else {
				l = await api.getLink(code);
			}
			link = l;
			editDestination = l.destination;
			editRemark = l.remark ?? '';
			editForwardUTM = l.forward_utm;
		} catch (e) {
			error = (e as Error).message;
			loading = false;
			return;
		}
		loading = false;
	});

	function applyRange(r: DateRange) {
		const url = new URL(page.url);
		if (r.preset === 'custom') {
			url.searchParams.set('from', r.from);
			url.searchParams.set('to', r.to);
			url.searchParams.delete('preset');
		} else {
			url.searchParams.set('preset', r.preset);
			url.searchParams.delete('from');
			url.searchParams.delete('to');
			localStorage.setItem(RANGE_KEY, r.preset);
		}
		goto(url.pathname + url.search, { keepFocus: true });
	}

	async function loadDimension(dimension: string, from: string, to: string): Promise<DashboardBreakdownItem[]> {
		const b = await api.getBreakdowns(code, dimension, from, to);
		const items: DashboardBreakdownItem[] = b.items.map((item) => {
			const e = Object.entries(item)[0];
			return { value: e?.[0] ?? 'unknown', visits: e?.[1] ?? 0, unique_visitors: 0 };
		});
		if (b.other > 0) {
			items.push({ value: 'Other', visits: b.other, unique_visitors: 0 });
		}
		return items;
	}

	async function loadAnalytics(from: string, to: string) {
		analyticsLoading = true;
		analyticsError = '';
		// Invalidate any in-flight per-tab fetches; the refetch owns all four
		// breakdown lists for the current tabs.
		srcSeq++;
		envSeq++;
		locSeq++;
		campSeq++;
		try {
			const [a, ts, src, env, loc, camp] = await Promise.all([
				api.getAnalytics(code, from, to),
				api.getTimeseries(code, from, to),
				loadDimension('referrer', from, to),
				loadDimension(envTab, from, to),
				loadDimension(locTab, from, to),
				loadDimension(campaignTab, from, to)
			]);
			analytics = a;
			timeseries = ts;
			sourceItems = src;
			envItems = env;
			locItems = loc;
			campaignItems = camp;
		} catch (e) {
			analyticsError = (e as Error).message;
		} finally {
			analyticsLoading = false;
		}
	}

	async function changeSourceTab(t: string) {
		sourceTab = t;
		const s = ++srcSeq;
		const items = await loadDimension(t, range.from, range.to);
		if (s === srcSeq) sourceItems = items;
	}

	async function changeEnvTab(t: string) {
		envTab = t;
		const s = ++envSeq;
		const items = await loadDimension(t, range.from, range.to);
		if (s === envSeq) envItems = items;
	}

	async function changeLocTab(t: string) {
		locTab = t;
		const s = ++locSeq;
		const items = await loadDimension(t, range.from, range.to);
		if (s === locSeq) locItems = items;
	}

	async function changeCampaignTab(t: string) {
		campaignTab = t;
		const s = ++campSeq;
		const items = await loadDimension(t, range.from, range.to);
		if (s === campSeq) campaignItems = items;
	}

	const sections: BreakdownSection[] = [
		{
			id: 'sources',
			label: 'Sources',
			fetcher: async () => ({ items: await loadDimension('referrer', range.from, range.to) }),
			metric: () => 'visits'
		},
		{
			id: 'environment',
			label: 'Environment',
			children: [
				{ id: 'browser', label: 'Browser' },
				{ id: 'os', label: 'OS' },
				{ id: 'device', label: 'Device' }
			],
			fetcher: async (sub) => ({
				items: await loadDimension(sub ?? 'browser', range.from, range.to)
			}),
			metric: () => 'visits'
		},
		{
			id: 'location',
			label: 'Location',
			children: [
				{ id: 'country', label: 'Country' },
				{ id: 'region', label: 'Region' },
				{ id: 'city', label: 'City' }
			],
			valueFormatter: (v, sub) => (sub === 'country' ? countryLabel(v) : v),
			fetcher: async (sub) => ({
				items: await loadDimension(sub ?? 'country', range.from, range.to)
			}),
			metric: () => 'visits'
		},
		{
			id: 'campaigns',
			label: 'Campaigns',
			children: [
				{ id: 'utm_source', label: 'UTM source' },
				{ id: 'utm_medium', label: 'UTM medium' },
				{ id: 'utm_campaign', label: 'UTM campaign' }
			],
			fetcher: async (sub) => ({
				items: await loadDimension(sub ?? 'utm_source', range.from, range.to)
			}),
			metric: () => 'visits'
		}
	];

	const dialogConfig = $derived(
		dialogOpen ? { sections, initial: dialogInitial } : null
	);

	function openDialog(id: string) {
		dialogInitial = id;
		dialogOpen = true;
	}

	async function copyShortUrl() {
		if (!link) return;
		try {
			await navigator.clipboard.writeText(shortUrl);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			/* clipboard unavailable */
		}
	}

	async function saveSettings() {
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
			await goto(backHref);
		} catch (e) {
			error = (e as Error).message;
		}
	}

	const totalVisits = $derived(analytics?.window.visits ?? 0);
	const totalUniques = $derived(analytics?.window.unique_visitors ?? 0);
	const lifetimeVisits = $derived(analytics?.lifetime.visits ?? 0);
	const spanDays = $derived(
		Math.max(1, Math.round((Date.parse(range.to) - Date.parse(range.from)) / 86_400_000) + 1)
	);
	const visitsPerDay = $derived(Math.round((totalVisits / spanDays) * 10) / 10);
</script>

<svelte:head>
	<title>{hostname ? `${hostname}/${code}` : 'Link'} - shrl.io</title>
</svelte:head>

{#if loading}
	<div class="space-y-6">
		<Skeleton class="h-9 w-72" />
		<Skeleton class="h-9 w-56" />
		<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
			{#each [0, 1, 2, 3] as i (i)}
				<Skeleton class="h-24 w-full" />
			{/each}
		</div>
		<Skeleton class="h-48 w-full" />
		<div class="grid gap-6 lg:grid-cols-2">
			<Skeleton class="h-64 w-full" />
			<Skeleton class="h-64 w-full" />
		</div>
	</div>
{:else if error}
	<Alert variant="destructive">
		<TriangleAlert class="size-4" />
		<AlertTitle>Failed to load Link</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{:else if link}
	<!-- Identity block: the short URL is the artifact, so it leads. -->
	<div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_auto]">
		<div class="min-w-0">
			<a
				href={backHref}
				class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground hover:underline"
			>
				<ChevronLeft class="size-4" /> {backLabel}
			</a>
			<div class="mt-3 flex flex-wrap items-center gap-3">
				<h1 class="min-w-0 break-all text-3xl font-semibold tracking-tight">
					{hostname}/{code}
				</h1>
				{#if link.disabled}
					<Badge variant="secondary">Disabled</Badge>
				{:else}
					<Badge>Active</Badge>
				{/if}
			</div>
			<div class="mt-4 flex flex-wrap gap-2">
				<Button onclick={copyShortUrl} class="gap-2">
					<Copy class="size-4" /> {copied ? 'Copied!' : 'Copy short URL'}
				</Button>
				<Button
					variant="outline"
					href={link.destination}
					target="_blank"
					rel="noopener"
					class="gap-2"
				>
					<ExternalLink class="size-4" /> Open destination
				</Button>
			</div>
			<p class="mt-3 text-sm text-muted-foreground">
				Created {link.created_at.slice(0, 10)} · Updated {link.updated_at.slice(0, 10)}
			</p>
			{#if teamId && !canManage}
				<p class="mt-1 text-sm text-muted-foreground">
					Read-only, managed by its Creator or a Team Owner.
				</p>
			{/if}
			{#if link.remark}
				<p class="mt-2 text-sm text-foreground/90">{link.remark}</p>
			{/if}
		</div>
		<div class="w-full lg:w-80">
			<LinkQR {hostname} {code} />
		</div>
	</div>

	<!-- Analytics: full dashboard parity scoped to this link. -->
	<div class="mt-8">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 class="text-lg font-semibold tracking-tight">Analytics</h2>
			<RangeSelect value={range} onchange={applyRange} />
		</div>
		{#if analyticsError}
			<Alert variant="destructive" class="mt-4">
				<TriangleAlert class="size-4" />
				<AlertDescription>{analyticsError}</AlertDescription>
			</Alert>
		{/if}
		<div class="mt-4 space-y-6">
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Lifetime visits
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading && !analytics ? '…' : lifetimeVisits}
						</p>
						<p class="text-xs text-muted-foreground">Never pruned</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Visits
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading && !analytics ? '…' : totalVisits}
						</p>
						<p class="text-xs text-muted-foreground">{presetLabel(range.preset)}</p>
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
							{analyticsLoading && !analytics ? '…' : totalUniques}
						</p>
						<p class="text-xs text-muted-foreground">{presetLabel(range.preset)}</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader class="pb-2">
						<CardTitle class="text-sm font-medium text-muted-foreground">
							Visits per day
						</CardTitle>
					</CardHeader>
					<CardContent class="pt-0">
						<p class="text-2xl font-semibold">
							{analyticsLoading && !analytics ? '…' : visitsPerDay}
						</p>
						<p class="text-xs text-muted-foreground">Average in range</p>
					</CardContent>
				</Card>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Visits & Visitors ({presetLabel(range.preset)})</CardTitle>
				</CardHeader>
				<CardContent>
					{#if analyticsLoading && !analytics}
						<Skeleton class="h-40 w-full" />
					{:else if totalVisits === 0}
						<div class="flex flex-col items-center gap-2 py-10 text-center">
							<Link2 class="size-6 text-muted-foreground" />
							<p class="text-sm font-medium">No visits in this period</p>
							<p class="max-w-xs text-xs text-muted-foreground">
								Share this link to start collecting visits. Bots and link-preview unfurlers
								are excluded.
							</p>
						</div>
					{:else}
						<StatsChart rows={bucketTimeseries(timeseries, range.from, range.to)} />
					{/if}
				</CardContent>
			</Card>

			<div class="grid gap-6 lg:grid-cols-2">
				<RankCard
					title="Sources"
					items={sourceItems}
					metric="visits"
					showVisitors={false}
					empty="No visits in this period."
					onMore={() => openDialog('sources')}
				/>
				<RankCard
					title="Environment"
					items={envItems}
					tabs={['browser', 'os', 'device']}
					active={envTab}
					onTabChange={changeEnvTab}
					metric="visits"
					showVisitors={false}
					empty="No visits in this period."
					onMore={() => openDialog('environment')}
				/>
			</div>
			<div class="grid gap-6 lg:grid-cols-2">
				<RankCard
					title="Location"
					items={locItems}
					tabs={['country', 'region', 'city']}
					active={locTab}
					onTabChange={changeLocTab}
					metric="visits"
					showVisitors={false}
					empty="No visits in this period."
					valueFormatter={(v) => (locTab === 'country' ? countryLabel(v) : v)}
					onMore={() => openDialog('location')}
				/>
				<RankCard
					title="Campaigns"
					items={campaignItems}
					tabs={[
						{ id: 'utm_source', label: 'source' },
						{ id: 'utm_medium', label: 'medium' },
						{ id: 'utm_campaign', label: 'campaign' }
					]}
					active={campaignTab}
					onTabChange={changeCampaignTab}
					metric="visits"
					showVisitors={false}
					empty="No visits in this period."
					onMore={() => openDialog('campaigns')}
				/>
			</div>

			<WorldMap from={range.from} to={range.to} {code} />
		</div>
	</div>

	<!-- Settings, with the destructive actions isolated at the bottom. -->
	<div class="mt-10">
		<Separator class="mb-6" />
		<h2 class="text-lg font-semibold tracking-tight">Settings</h2>
		<div class="mt-4 space-y-6">
			<Card class="max-w-2xl">
				<CardHeader>
					<CardTitle>Destination & remark</CardTitle>
					<CardDescription>
						Where this Link redirects to, and the note for it.
					</CardDescription>
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
							saveSettings();
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
							<Checkbox
								id="forward-utm"
								bind:checked={editForwardUTM}
								disabled={!canManage}
								class="mt-0.5"
							/>
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
				<Card class="max-w-2xl border-destructive/30">
					<CardHeader>
						<CardTitle>Danger zone</CardTitle>
						<CardDescription>
							{link.disabled
								? 'This Link currently returns 404 from the redirector. Enable restores redirects.'
								: 'This Link redirects visitors to its Destination. Disable or delete to stop it.'}
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
		</div>
	</div>

	<BreakdownDialog
		open={dialogOpen}
		config={dialogConfig}
		showVisitors={false}
		onclose={() => (dialogOpen = false)}
	/>
{/if}
