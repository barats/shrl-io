<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import type { DashboardBreakdownItem } from '$lib/types';
	import { X } from '@lucide/svelte';

	export interface BreakdownDialogConfig {
		title: string;
		fetcher: () => Promise<{
			items: DashboardBreakdownItem[];
			hrefs?: Record<string, string>;
		}>;
		valueFormatter?: (value: string) => string;
		metric?: 'visits' | 'visitors';
	}

	let {
		open,
		config,
		rangeKey,
		onclose
	}: {
		open: boolean;
		config: BreakdownDialogConfig | null;
		// Changes (e.g. the selected range) refetch an open dialog.
		rangeKey: string;
		onclose: () => void;
	} = $props();

	let items = $state<DashboardBreakdownItem[]>([]);
	let hrefs = $state<Record<string, string>>({});
	let loading = $state(false);
	let error = $state('');

	$effect(() => {
		if (!open || !config) return;
		// Re-read the live range: rangeKey changes on range change so an open
		// dialog refetches; the fetcher closes over the reactive range state.
		void rangeKey;
		loading = true;
		error = '';
		items = [];
		config
			.fetcher()
			.then((res) => {
				items = res.items;
				hrefs = res.hrefs ?? {};
			})
			.catch((e) => {
				error = (e as Error).message;
			})
			.finally(() => {
				loading = false;
			});
	});

	$effect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onclose();
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	const metric = $derived(config?.metric ?? 'visitors');
	const metricKey = $derived(metric === 'visits' ? 'visits' : 'unique_visitors');
	const maxMetric = $derived(Math.max(1, ...items.map((it) => it[metricKey])));
	const display = $derived(
		(v: string) => (config?.valueFormatter ? config.valueFormatter(v) : v)
	);
</script>

{#if open && config}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		aria-label={config.title}
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) onclose();
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') onclose();
		}}
	>
		<Card class="flex max-h-[80vh] w-full max-w-lg flex-col">
			<CardHeader class="flex-row items-center justify-between space-y-0">
				<CardTitle>{config.title}</CardTitle>
				<Button
					variant="ghost"
					size="icon-sm"
					aria-label="Close"
					onclick={onclose}
				>
					<X class="size-4" />
				</Button>
			</CardHeader>
			<CardContent class="flex-1 overflow-y-auto">
				{#if loading}
					<Skeleton class="h-40 w-full" />
				{:else if error}
					<p class="py-4 text-center text-sm text-destructive">{error}</p>
				{:else if items.length === 0}
					<p class="py-4 text-center text-sm text-muted-foreground">
						No visits in this period.
					</p>
				{:else}
					<div class="space-y-3">
						{#each items as item (item.value)}
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
			</CardContent>
		</Card>
	</div>
{/if}
