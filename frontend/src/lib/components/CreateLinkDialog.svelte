<script lang="ts">
	import { api } from '$lib/api';
	import { Alert, AlertDescription } from '$lib/components/ui/alert';
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
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select';
	import { Plus, TriangleAlert } from '@lucide/svelte';

	let {
		open = $bindable(false),
		baseURLs,
		defaultBaseURL,
		teamId = undefined,
		onCreated
	}: {
		open: boolean;
		baseURLs: string[];
		defaultBaseURL: string;
		teamId?: number;
		onCreated?: () => void;
	} = $props();

	let createBaseURL = $state('');
	let createDestination = $state('');
	let createRemark = $state('');
	let createForwardUTM = $state(false);
	let creating = $state(false);
	let error = $state('');

	$effect(() => {
		if (!open) return;
		createBaseURL = defaultBaseURL;
		createDestination = '';
		createRemark = '';
		createForwardUTM = false;
		error = '';
	});

	$effect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') open = false;
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	async function create() {
		creating = true;
		error = '';
		try {
			const input = {
				base_url: createBaseURL,
				destination: createDestination,
				remark: createRemark || undefined,
				forward_utm: createForwardUTM
			};
			if (teamId !== undefined) {
				await api.createTeamLink(teamId, input);
			} else {
				await api.createLink(input);
			}
			open = false;
			onCreated?.();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			creating = false;
		}
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		aria-label="Create a Link"
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) open = false;
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') open = false;
		}}
	>
		<Card class="w-full max-w-md">
			<CardHeader>
				<CardTitle>Create a Link</CardTitle>
				<CardDescription>
					{teamId !== undefined
						? 'Shorten a Destination under a registered Base URL. The Team owns the Link.'
						: "Shorten a Destination under this instance's registered Base URLs."}
				</CardDescription>
			</CardHeader>
			<CardContent>
				{#if error}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertDescription>{error}</AlertDescription>
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
						<Label for="dlg-base-url">Base URL</Label>
						<Select type="single" bind:value={createBaseURL}>
							<SelectTrigger id="dlg-base-url" class="w-full">
								<span data-slot="select-value">{createBaseURL || 'Base URL'}</span>
							</SelectTrigger>
							<SelectContent>
								{#each baseURLs as url (url)}
									<SelectItem value={url} label={url} />
								{/each}
							</SelectContent>
						</Select>
					</div>
					<div class="space-y-2">
						<Label for="dlg-destination">Destination</Label>
						<Input
							id="dlg-destination"
							bind:value={createDestination}
							placeholder="https://example.com"
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="dlg-remark">Remark (optional)</Label>
						<Input
							id="dlg-remark"
							bind:value={createRemark}
							placeholder="What this Link is for"
						/>
					</div>
					<div class="flex items-start gap-2">
						<Checkbox id="dlg-forward-utm" bind:checked={createForwardUTM} class="mt-0.5" />
						<Label
							for="dlg-forward-utm"
							class="font-normal leading-snug text-muted-foreground"
						>
							Forward UTM parameters from the short URL to the Destination
						</Label>
					</div>
					<div class="flex justify-end gap-2 pt-2">
						<Button type="button" variant="outline" onclick={() => (open = false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={creating}>
							{#if creating}
								Creating…
							{:else}
								<Plus class="size-4" /> Create Link
							{/if}
						</Button>
					</div>
				</form>
			</CardContent>
		</Card>
	</div>
{/if}
