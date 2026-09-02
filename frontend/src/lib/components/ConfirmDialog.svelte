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
	import * as Dialog from '$lib/components/ui/dialog';
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

<Dialog.Root open={request !== null} onOpenChange={(o) => (o ? undefined : onclose())}>
	{#if request}
		<Dialog.Content class="sm:max-w-sm">
			<div class="flex items-start gap-3">
				{#if request.destructive}
					<div
						class="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-destructive/10 text-destructive"
					>
						<TriangleAlert class="size-4" />
					</div>
				{/if}
				<div class="min-w-0">
					<Dialog.Title>{request.title}</Dialog.Title>
					<Dialog.Description>{request.description}</Dialog.Description>
				</div>
			</div>
			<Dialog.Footer>
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
			</Dialog.Footer>
		</Dialog.Content>
	{/if}
</Dialog.Root>
