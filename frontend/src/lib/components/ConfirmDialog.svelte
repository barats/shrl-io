<script lang="ts" module>
	export type ConfirmRequest = {
		title: string;
		description: string;
		confirmLabel: string;
		destructive?: boolean;
		action: () => void | Promise<void>;
	};
</script>

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { TriangleAlert } from '@lucide/svelte';

	// A small in-app confirmation dialog for destructive or state-changing
	// actions (delete, disable), replacing native confirm().
	let {
		request,
		onclose
	}: {
		request: ConfirmRequest | null;
		onclose: () => void;
	} = $props();

	let confirming = $state(false);

	$effect(() => {
		// Reset the in-flight state whenever a new request opens.
		request;
		confirming = false;
	});

	async function confirm() {
		if (!request) return;
		confirming = true;
		try {
			await request.action();
			onclose();
		} finally {
			confirming = false;
		}
	}
</script>

{#if request}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		aria-label={request.title}
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) onclose();
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') onclose();
		}}
	>
		<Card class="w-full max-w-sm">
			<CardHeader>
				<div class="flex items-start gap-3">
					{#if request.destructive}
						<div
							class="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-destructive/10 text-destructive"
						>
							<TriangleAlert class="size-4" />
						</div>
					{/if}
					<div class="min-w-0">
						<CardTitle class="text-base">{request.title}</CardTitle>
						<p class="mt-1 text-sm text-muted-foreground">{request.description}</p>
					</div>
				</div>
			</CardHeader>
			<CardFooter class="flex justify-end gap-2">
				<Button type="button" variant="outline" onclick={onclose}>
					Cancel
				</Button>
				<Button
					type="button"
					variant={request.destructive ? 'destructive' : 'default'}
					disabled={confirming}
					onclick={confirm}
				>
					{confirming ? `${request.confirmLabel}…` : request.confirmLabel}
				</Button>
			</CardFooter>
		</Card>
	</div>
{/if}
