<script lang="ts">
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import type { DashboardBreakdownItem } from '$lib/types';

	// Number of rows a card shows before the "More" button takes over.
	const visibleRows = 7;

	let {
		title,
		items,
		empty = 'No visits in this period.',
		tabs = [],
		active,
		onTabChange,
		valueFormatter,
		hrefs = {},
		metric = 'visitors',
		onMore
	}: {
		title: string;
		items: DashboardBreakdownItem[];
		empty?: string;
		tabs?: string[];
		active?: string;
		onTabChange?: (tab: string) => void;
		valueFormatter?: (value: string) => string;
		// value -> href; a present key renders the row label as a link.
		hrefs?: Record<string, string>;
		// Which total the bar is scaled to: the ordering metric.
		metric?: 'visits' | 'visitors';
		// Opened by the centered "More" button, which always renders and opens
		// the full breakdown dialog for this card's section.
		onMore?: () => void;
	} = $props();

	const display = $derived((v: string) => (valueFormatter ? valueFormatter(v) : v));
	const metricKey = $derived(metric === 'visits' ? 'visits' : 'unique_visitors');
	const maxMetric = $derived(Math.max(1, ...items.map((it) => it[metricKey])));
	const shown = $derived(items.slice(0, visibleRows));
</script>

<Card class="flex h-full flex-col">
	<CardHeader class="flex-row items-center justify-between space-y-0">
		<CardTitle>{title}</CardTitle>
		{#if tabs.length > 1}
			<div class="flex items-center gap-1 rounded-md bg-muted p-1">
				{#each tabs as tab (tab)}
					<button
						type="button"
						class="rounded px-2 py-1 text-xs font-medium transition-colors {active === tab
							? 'bg-background text-foreground shadow-sm'
							: 'text-muted-foreground hover:text-foreground'}"
						onclick={() => onTabChange?.(tab)}
					>
						{tab}
					</button>
				{/each}
			</div>
		{/if}
	</CardHeader>
	<CardContent class="flex flex-1 flex-col">
		<div class="min-h-[300px] flex-1">
			{#if items.length === 0}
				<p class="py-4 text-center text-sm text-muted-foreground">{empty}</p>
			{:else}
				<div class="space-y-3">
					{#each shown as item (item.value)}
						<div>
							<div class="flex items-center justify-between gap-2 text-sm">
								<span class="truncate font-medium">
									{#if hrefs[item.value]}
										<a href={hrefs[item.value]} class="text-primary hover:underline">
											{display(item.value)}
										</a>
									{:else}
										{display(item.value)}
									{/if}
								</span>
								<span class="shrink-0 text-xs text-muted-foreground">
									{item.visits} visits · {item.unique_visitors} visitors
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
			class="mt-3 flex w-full items-center justify-center rounded-md py-1.5 text-sm font-medium text-primary hover:bg-muted"
		>
			More
		</button>
	</CardContent>
</Card>
