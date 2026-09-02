<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import VisitsEmptyState from '$lib/components/VisitsEmptyState.svelte';
	import type { DashboardBreakdownItem } from '$lib/types';
	import { ChevronLeft, ChevronRight, Menu, X } from '@lucide/svelte';

	// One entry in the dialog's left nav: a dashboard section whose full
	// breakdown renders in the right pane. A section without `children` is its
	// own single view (e.g. Sources); one with `children` (e.g. Environment's
	// browser/os/device) nests them under the parent nav item, always visible.
	export interface BreakdownSection {
		id: string;
		label: string;
		children?: BreakdownSubsection[];
		fetcher: (sub?: string) => Promise<{
			items: DashboardBreakdownItem[];
			hrefs?: Record<string, string>;
		}>;
		valueFormatter?: (value: string, sub?: string) => string;
		// Which metric each row's count and bar is based on; can depend on the
		// selected sub-category (Top Links' visits vs visitors).
		metric?: (sub?: string) => 'visits' | 'visitors';
	}

	export interface BreakdownSubsection {
		id: string;
		label: string;
	}

	export interface BreakdownDialogConfig {
		sections: BreakdownSection[];
		// Section preselected on open: the card whose "More" button was clicked.
		initial: string;
	}

	let {
		open,
		config,
		onclose,
		// False when only visit counts are available (per-link breakdowns);
		// drops the Visitors column from the table.
		showVisitors = true
	}: {
		open: boolean;
		config: BreakdownDialogConfig | null;
		onclose: () => void;
		showVisitors?: boolean;
	} = $props();

	// The dialog's own selection, deliberately decoupled from the cards' tabs:
	// switching sub-categories here never changes the tabs on the dashboard.
	let selected = $state<string>('');
	let selectedChild = $state<string | undefined>(undefined);
	// Remember each parent's last sub-category for the open session.
	let lastChild = $state<Record<string, string>>({});
	let items = $state<DashboardBreakdownItem[]>([]);
	let hrefs = $state<Record<string, string>>({});
	let loading = $state(false);
	let error = $state('');
	let page = $state(0);
	let dialogEl = $state<HTMLDivElement | null>(null);
	// Mobile: the left nav slides in as a drawer behind a hamburger button.
	let navOpen = $state(false);
	// Bumped per fetch so a slow response for a previously-selected section
	// can't overwrite the section the user has since navigated to. Plain (not
	// $state): the fetch effect must not re-run when it increments this.
	let fetchSeq = 0;

	const PAGE_SIZE = 15;

	function section(id: string): BreakdownSection | undefined {
		return config?.sections.find((s) => s.id === id);
	}

	function selectParent(id: string, child?: string) {
		selected = id;
		const sec = section(id);
		if (sec?.children?.length) {
			selectedChild = child ?? lastChild[id] ?? sec.children[0].id;
		} else {
			selectedChild = undefined;
		}
	}

	function selectChild(id: string, childId: string) {
		selected = id;
		selectedChild = childId;
		lastChild[id] = childId;
	}

	// Reset to the clicked section each time the dialog opens.
	$effect(() => {
		if (!open || !config) return;
		selectParent(config.initial);
	});

	// Fetch on demand: navigating the nav (or opening) fetches only that
	// section's rows.
	$effect(() => {
		if (!open || !config) return;
		const sec = section(selected);
		if (!sec) return;
		const seq = ++fetchSeq;
		page = 0;
		loading = true;
		error = '';
		items = [];
		hrefs = {};
		sec
			.fetcher(selectedChild)
			.then((res) => {
				if (seq !== fetchSeq) return;
				items = res.items;
				hrefs = res.hrefs ?? {};
			})
			.catch((e) => {
				if (seq !== fetchSeq) return;
				error = (e as Error).message;
			})
			.finally(() => {
				if (seq === fetchSeq) loading = false;
			});
	});

	// Close on Escape, lock page scroll, move focus in, and trap Tab inside
	// the full-screen surface so focus cannot escape to the page behind it.
	$effect(() => {
		if (!open) return;
		dialogEl?.focus();
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				if (navOpen) {
					navOpen = false;
				} else {
					onclose();
				}
				return;
			}
			if (e.key === 'Tab' && dialogEl) {
				// Only the visible focusables are reachable by Tab; hidden ones
				// (e.g. the desktop sidebar on mobile) would let focus escape.
				const focusables = [
					...dialogEl.querySelectorAll<HTMLElement>(
						'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'
					)
				].filter((el) => el.offsetParent !== null);
				if (focusables.length === 0) return;
				const idx = focusables.indexOf(document.activeElement as HTMLElement);
				const next = e.shiftKey
					? (idx <= 0 ? focusables.length - 1 : idx - 1)
					: (idx === -1 || idx === focusables.length - 1 ? 0 : idx + 1);
				e.preventDefault();
				focusables[next].focus();
			}
		};
		window.addEventListener('keydown', onKey);
		document.body.style.overflow = 'hidden';
		return () => {
			window.removeEventListener('keydown', onKey);
			document.body.style.overflow = '';
		};
	});

	const current = $derived(config ? section(selected) : undefined);
	const currentChild = $derived(current?.children?.find((c) => c.id === selectedChild));
	const metric = $derived(current?.metric?.(selectedChild) ?? 'visitors');
	const metricKey = $derived(metric === 'visits' ? 'visits' : 'unique_visitors');
	const total = $derived(items.reduce((n, it) => n + it[metricKey], 0));
	const display = $derived((v: string) => current?.valueFormatter?.(v, selectedChild) ?? v);
	const pct = $derived((it: DashboardBreakdownItem) =>
		total === 0 ? 0 : Math.round((it[metricKey] / total) * 100)
	);
	const pageCount = $derived(Math.max(1, Math.ceil(items.length / PAGE_SIZE)));
	const pageItems = $derived(items.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE));
	const rangeStart = $derived(items.length === 0 ? 0 : page * PAGE_SIZE + 1);
	const rangeEnd = $derived(Math.min(items.length, (page + 1) * PAGE_SIZE));
	const fmt = $derived((n: number) => n.toLocaleString('en-US'));

	function handleSectionClick(id: string) {
		if (id !== selected) selectParent(id);
		navOpen = false;
	}

	function handleChildClick(id: string, childId: string) {
		selectChild(id, childId);
		navOpen = false;
	}
</script>

{#if open && config}
	{#snippet navItems()}
		{#each config.sections as sec (sec.id)}
			<div>
				<button
					type="button"
					class="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm font-medium transition-colors {selected === sec.id
						? 'bg-muted text-foreground'
						: 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'}"
					onclick={() => handleSectionClick(sec.id)}
				>
					{sec.label}
				</button>
				{#if sec.children?.length}
					<div class="ml-3 mt-1 flex flex-col gap-0.5 border-l pl-2">
						{#each sec.children as child (child.id)}
							<button
								type="button"
								class="rounded-md px-2 py-1 text-left text-xs transition-colors {selectedChild === child.id && selected === sec.id
									? 'bg-muted font-medium text-foreground'
									: 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'}"
								onclick={() => handleChildClick(sec.id, child.id)}
							>
								{child.label}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		{/each}
	{/snippet}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="presentation"
		onclick={(e) => {
			if (e.target === e.currentTarget) onclose();
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') onclose();
		}}
	>
		<!-- The modal is full-height (constant regardless of data) but centered,
		     so it reads as a page inside a modal frame. -->
		<div
			class="relative flex h-[calc(100vh-2rem)] w-full max-w-5xl flex-col overflow-hidden rounded-lg bg-background shadow-xl ring-1 ring-border"
			role="dialog"
			aria-modal="true"
			aria-label="Breakdown details"
			tabindex="-1"
			bind:this={dialogEl}
		>
		<!-- Fixed header: title, mobile menu, close. -->
		<header class="flex shrink-0 items-center gap-2 border-b px-3 py-3 md:px-5">
			<Button
				variant="ghost"
				size="icon-sm"
				class="md:hidden"
				aria-label="Open sections menu"
				onclick={() => (navOpen = true)}
			>
				<Menu class="size-4" />
			</Button>
			<h2 class="min-w-0 flex-1 truncate text-lg font-semibold tracking-tight">
				{current?.label}{#if currentChild}&nbsp;-&nbsp;{currentChild.label}{/if}
			</h2>
			<Button
				variant="ghost"
				size="icon-sm"
				aria-label="Close"
				onclick={onclose}
			>
				<X class="size-4" />
			</Button>
		</header>

		<!-- Mobile: horizontal top-level section chips. -->
		<div class="flex shrink-0 gap-1.5 overflow-x-auto border-b px-3 py-2 md:hidden">
			{#each config.sections as sec (sec.id)}
				<button
					type="button"
					class="whitespace-nowrap rounded-full px-3 py-1 text-xs font-medium transition-colors {selected === sec.id
						? 'bg-primary text-primary-foreground'
						: 'bg-muted text-muted-foreground hover:bg-muted/70'}"
					onclick={() => handleSectionClick(sec.id)}
				>
					{sec.label}
				</button>
			{/each}
		</div>

		<!-- Body: fixed left nav + scrolling main area. -->
		<div class="flex min-h-0 flex-1">
			<nav
				class="hidden w-60 shrink-0 overflow-y-auto border-r p-3 md:block"
				aria-label="Breakdown sections"
			>
				{@render navItems()}
			</nav>
			<div class="flex min-w-0 flex-1 flex-col">
				<div class="min-h-0 flex-1 overflow-y-auto p-4 md:p-5">
					{#if loading}
						<Skeleton class="h-40 w-full" />
					{:else if error}
						<p class="py-4 text-center text-sm text-destructive">{error}</p>
					{:else if items.length === 0}
						<VisitsEmptyState compact />
					{:else}
						<div class="rounded-md border">
							<div
								class="grid items-center gap-3 border-b px-4 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground {showVisitors
									? 'grid-cols-[minmax(0,1fr)_auto_auto]'
									: 'grid-cols-[minmax(0,1fr)_auto]'}"
							>
								<span>Label</span>
								<span class="min-w-14 text-right">Visits</span>
								{#if showVisitors}
									<span class="min-w-14 text-right">Visitors</span>
								{/if}
							</div>
							{#each pageItems as item (item.value)}
								<div
									class="relative grid items-center gap-3 border-b px-4 py-2 last:border-b-0 {showVisitors
										? 'grid-cols-[minmax(0,1fr)_auto_auto]'
										: 'grid-cols-[minmax(0,1fr)_auto]'}"
								>
									<div
										class="absolute inset-y-0 left-0 bg-primary/10"
										style="width: {pct(item)}%"
									></div>
									<span class="relative truncate text-sm font-medium">
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
									<span class="relative min-w-14 text-right text-sm tabular-nums">
										{fmt(item.visits)}
									</span>
									{#if showVisitors}
										<span class="relative min-w-14 text-right text-sm tabular-nums text-muted-foreground">
											{fmt(item.unique_visitors)}
										</span>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
				{#if items.length > PAGE_SIZE}
					<div class="flex shrink-0 items-center justify-between border-t px-4 py-3 text-sm text-muted-foreground md:px-5">
						<span>Showing {rangeStart}-{rangeEnd} of {items.length}</span>
						<div class="flex items-center gap-2">
							<Button
								variant="outline"
								size="sm"
								disabled={page === 0}
								onclick={() => (page = page - 1)}
							>
								<ChevronLeft class="size-4" /> Prev
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={page >= pageCount - 1}
								onclick={() => (page = page + 1)}
							>
								Next <ChevronRight class="size-4" />
							</Button>
						</div>
					</div>
				{/if}
			</div>
		</div>

		<!-- Mobile drawer: the full nav (sections + sub-categories). -->
		{#if navOpen}
			<div
				class="absolute inset-0 z-20 bg-black/50 md:hidden"
				role="presentation"
				aria-hidden="true"
				onclick={() => (navOpen = false)}
				onkeydown={() => {}}
			></div>
			<nav
				class="absolute inset-y-0 left-0 z-30 flex w-72 flex-col border-r bg-background p-3 md:hidden"
				aria-label="Breakdown sections"
			>
				<div class="mb-2 flex items-center justify-between">
					<span class="text-sm font-semibold">Breakdowns</span>
					<Button
						variant="ghost"
						size="icon-xs"
						aria-label="Close menu"
						onclick={() => (navOpen = false)}
					>
						<X class="size-4" />
					</Button>
				</div>
				<div class="min-h-0 flex-1 overflow-y-auto">
					{@render navItems()}
				</div>
			</nav>
		{/if}
		</div>
	</div>
{/if}
