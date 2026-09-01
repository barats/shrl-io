<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { Link } from '$lib/types';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Check, Copy, Link2, Search, TriangleAlert } from '@lucide/svelte';

	// The shared management list for both Personal and Team Links. The pages
	// own the data loading and the create flow; this component owns the
	// toolbar, filtering, rows, and the empty/loading/error states so the two
	// surfaces cannot drift.
	let {
		links,
		loading,
		error = '',
		hrefPrefix,
		searchable = true
	}: {
		links: Link[];
		loading: boolean;
		error?: string;
		hrefPrefix: string;
		searchable?: boolean;
	} = $props();

	let query = $state('');
	let copiedCode = $state('');
	let copyTimer: ReturnType<typeof setTimeout> | undefined;

	onDestroy(() => {
		if (copyTimer) clearTimeout(copyTimer);
	});

	const activeCount = $derived(links.filter((l) => !l.disabled).length);

	const filtered = $derived(
		query.trim() === ''
			? links
			: links.filter((l) => {
					const q = query.trim().toLowerCase();
					return (
						`${l.base_url}/${l.code}`.toLowerCase().includes(q) ||
						l.destination.toLowerCase().includes(q) ||
						l.remark.toLowerCase().includes(q)
					);
				})
	);

	// Relative date computed in UTC to match created_at's sliced date portion
	// (a UTC timestamp rendered without a timezone shift).
	function relativeDate(iso: string): string {
		const [y, m, d] = iso.slice(0, 10).split('-').map(Number);
		if (!y || !m || !d) return iso.slice(0, 10);
		const now = new Date();
		const today = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
		const day = Date.UTC(y, m - 1, d);
		const days = Math.round((today - day) / 86_400_000);
		if (days <= 0) return 'Today';
		if (days === 1) return 'Yesterday';
		if (days < 7) return `${days}d`;
		if (days < 30) return `${Math.floor(days / 7)}w`;
		if (days < 365) return `${Math.floor(days / 30)}mo`;
		return `${Math.floor(days / 365)}y`;
	}

	async function copyShortUrl(link: Link) {
		try {
			await navigator.clipboard.writeText(`${link.base_url}/${link.code}`);
			copiedCode = link.code;
			if (copyTimer) clearTimeout(copyTimer);
			copyTimer = setTimeout(() => (copiedCode = ''), 2000);
		} catch {
			/* clipboard unavailable */
		}
	}
</script>

<div class="mt-4 space-y-6">
	{#if error}
		<Alert variant="destructive">
			<TriangleAlert class="size-4" />
			<AlertTitle>Failed to load Links</AlertTitle>
			<AlertDescription>{error}</AlertDescription>
		</Alert>
	{/if}

	{#if loading}
		<div class="space-y-3">
			{#each [0, 1, 2, 3, 4] as i (i)}
				<div class="flex items-center gap-4 py-2">
					<div class="flex-1 space-y-1.5">
						<Skeleton class="h-4 w-40" />
						<Skeleton class="h-3 w-64" />
					</div>
					<Skeleton class="h-4 w-12" />
				</div>
			{/each}
		</div>
	{:else if links.length === 0}
		<Card>
			<CardContent class="flex flex-col items-center gap-3 py-10 text-center">
				<Link2 class="size-8 text-muted-foreground" />
				<h2 class="text-lg font-semibold">No Links yet</h2>
				<p class="max-w-xs text-sm text-muted-foreground">
					Shorten a Destination under one of the registered Base URLs with the
					Create Link button above.
				</p>
			</CardContent>
		</Card>
	{:else}
		<Card>
			<CardContent class="pt-4">
				<div class="flex flex-wrap items-center justify-between gap-3 pb-2">
					<p class="text-sm text-muted-foreground">
						{links.length} {links.length === 1 ? 'link' : 'links'} · {activeCount} active
					</p>
					{#if searchable}
						<div class="relative w-full max-w-64">
							<Search
								class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
							/>
							<Input
								bind:value={query}
								type="search"
								placeholder="Search codes, URLs, remarks"
								class="pl-8"
								aria-label="Search links"
							/>
						</div>
					{/if}
				</div>

				{#if filtered.length === 0}
					<div class="flex flex-col items-center gap-2 py-10 text-center">
						<Search class="size-6 text-muted-foreground" />
						<p class="text-sm font-medium">No Links match your search</p>
						<Button variant="ghost" size="sm" onclick={() => (query = '')}>
							Clear search
						</Button>
					</div>
				{:else}
					<div class="divide-y">
						{#each filtered as link (link.code)}
							<div
								class="group flex flex-col gap-1 py-3 transition-colors hover:bg-muted/40 md:flex-row md:items-center md:gap-4 md:px-2 md:py-2.5"
							>
								<div class="min-w-0 flex-1">
									<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
										<a
											href={`${hrefPrefix}/${encodeURIComponent(link.code)}`}
											class="font-mono text-sm font-medium text-foreground hover:text-primary hover:underline"
										>
											{link.base_url}/{link.code}
										</a>
										{#if link.disabled}
											<Badge variant="secondary">Disabled</Badge>
										{/if}
									</div>
									<div class="truncate text-xs text-muted-foreground">
										{link.destination}
									</div>
									{#if link.remark}
										<div class="truncate text-xs text-muted-foreground/80">
											{link.remark}
										</div>
									{/if}
								</div>
								<div class="flex items-center justify-between gap-4 md:justify-end">
									<span
										class="shrink-0 text-xs tabular-nums text-muted-foreground"
										title={link.created_at.slice(0, 10)}
									>
										{relativeDate(link.created_at)}
									</span>
									<Button
										variant="ghost"
										size="icon-sm"
										onclick={() => copyShortUrl(link)}
										aria-label={copiedCode === link.code ? 'Copied' : 'Copy short URL'}
										title="Copy short URL"
									>
										{#if copiedCode === link.code}
											<Check class="size-4 text-green-600" />
										{:else}
											<Copy class="size-4" />
										{/if}
									</Button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</CardContent>
		</Card>
	{/if}
</div>
