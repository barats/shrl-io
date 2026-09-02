<script lang="ts">
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import VisitsEmptyState from '$lib/components/VisitsEmptyState.svelte';
	import type { DashboardBreakdownItem } from '$lib/types';

	// Number of rows a card shows before the "More" button takes over.
	const visibleRows = 7;

	let {
		title,
		items,
		tabs = [],
		active,
		onTabChange,
		valueFormatter,
		hrefs = {},
		metric = 'visitors',
		showVisitors = true,
		showRank = false,
		onMore
	}: {
		title: string;
		items: DashboardBreakdownItem[];
		// A tab is either a plain id (rendered as its own label) or an
		// id/label pair for display names that differ from the id.
		tabs?: Array<string | { id: string; label: string }>;
		active?: string;
		onTabChange?: (tab: string) => void;
		valueFormatter?: (value: string) => string;
		// value -> href; a present key renders the row label as a link.
		hrefs?: Record<string, string>;
		// Which total the bar is scaled to: the ordering metric.
		metric?: 'visits' | 'visitors';
		// False when only visit counts are available (per-link breakdowns);
		// hides the visitor count on each row.
		showVisitors?: boolean;
		// Prepend a 1-based rank number to each row (ranked lists like Top Links).
		showRank?: boolean;
		// Opened by the centered "More" button, which always renders and opens
		// the full breakdown dialog for this card's section.
		onMore?: () => void;
	} = $props();

	const display = $derived((v: string) => (valueFormatter ? valueFormatter(v) : v));
	const metricKey = $derived(metric === 'visits' ? 'visits' : 'unique_visitors');
	const maxMetric = $derived(Math.max(1, ...items.map((it) => it[metricKey])));
	const shown = $derived(items.slice(0, visibleRows));
	const showVisitorCount = $derived(showVisitors === true);
</script>

<Card class="flex h-full flex-col">
	<CardHeader class="flex-row items-center justify-between space-y-0">
		<CardTitle>{title}</CardTitle>
		{#if tabs.length > 1}
			<div class="flex items-center gap-1 rounded-md bg-muted p-1">
				{#each tabs as tab (tab)}
					{@const id = typeof tab === 'string' ? tab : tab.id}
					{@const label = typeof tab === 'string' ? tab : tab.label}
					<button
						type="button"
						class="rounded px-2 py-1 text-xs font-medium transition-colors {active === id
							? 'bg-background text-foreground shadow-sm'
							: 'text-foreground/70 hover:text-foreground'}"
						onclick={() => onTabChange?.(id)}
					>
						{label}
					</button>
				{/each}
			</div>
		{/if}
	</CardHeader>
	<CardContent class="flex flex-1 flex-col">
		<div class="min-h-[300px] flex-1">
			{#if items.length === 0}
				<VisitsEmptyState compact />
			{:else}
				<div class="space-y-3">
					{#each shown as item, i (item.value)}
						<div>
							<div class="flex items-center justify-between gap-2 text-sm">
								<span class="flex min-w-0 items-center gap-2">
									{#if showRank}
										<span class="shrink-0 text-xs font-medium tabular-nums text-muted-foreground">
											{i + 1}
										</span>
									{/if}
									<span class="truncate font-medium">
										{#if hrefs[item.value]}
											<a
												href={hrefs[item.value]}
												class="text-link hover:underline {item.label ? 'font-mono text-[13px]' : ''}"
												title={item.label ? item.value : undefined}
											>
												{item.label ?? display(item.value)}
											</a>
										{:else}
											{item.label ?? display(item.value)}
										{/if}
									</span>
								</span>
								<span class="shrink-0 text-xs text-muted-foreground">
									{item.visits} visits{#if showVisitorCount} · {item.unique_visitors} visitors{/if}
								</span>
							</div>
							<div class="mt-1 h-1.5 w-full rounded-full bg-muted">
								<div
									class="h-1.5 rounded-full bg-primary"
									style="width: {(item[metricKey] / maxMetric) * 100}%"
								></div>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
		<button
			type="button"
			onclick={onMore}
			class="mt-3 flex w-full items-center justify-center rounded-md py-1.5 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
		>
			More
		</button>
	</CardContent>
</Card>
