<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import { api } from '$lib/api';
	import type { User } from '$lib/types';
	import ConfirmDialog, { type ConfirmRequest } from '$lib/components/ConfirmDialog.svelte';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Check, LogOut, Save, Trash2, TriangleAlert } from '@lucide/svelte';

	const teamId = $derived(Number(page.params.id));
	const team = $derived(page.data.team);
	const myRole = $derived(page.data.myRole);
	const isAdmin = $derived(page.data.user?.isAdmin ?? false);
	const canRename = $derived(myRole === 'owner' || isAdmin);

	let me = $state<User | null>(null);
	let error = $state('');

	// In-app confirm dialog for destructive actions (replaces native confirm()).
	let confirmRequest = $state<ConfirmRequest | null>(null);

	let renameName = $state('');
	let renaming = $state(false);
	let renameError = $state('');
	let renamed = $state(false);

	onMount(async () => {
		renameName = team?.name ?? '';
		try {
			me = await api.me();
		} catch (e) {
			error = (e as Error).message;
		}
	});

	async function rename() {
		renaming = true;
		renameError = '';
		renamed = false;
		try {
			await api.renameTeam(teamId, renameName.trim());
			renamed = true;
			await invalidateAll();
		} catch (e) {
			renameError = (e as Error).message;
		} finally {
			renaming = false;
		}
	}

	function leave() {
		if (!me) return;
		const meId = me.id;
		confirmRequest = {
			title: 'Leave this Team?',
			description: 'Your Links stay with the Team. You can rejoin with a new invite code.',
			confirmLabel: 'Leave',
			action: async () => {
				try {
					await api.removeTeamMember(teamId, meId);
					await goto('/teams');
				} catch (e) {
					error = (e as Error).message;
				}
			}
		};
	}

	function removeTeam() {
		confirmRequest = {
			title: 'Delete this Team?',
			description: 'Its Links revert to Personal for their Creators. This cannot be undone.',
			confirmLabel: 'Delete',
			destructive: true,
			action: async () => {
				try {
					await api.deleteTeam(teamId);
					await goto('/teams');
				} catch (e) {
					error = (e as Error).message;
				}
			}
		};
	}
</script>

<svelte:head>
	<title>Settings - {team?.name ?? 'Team'} - shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>

{#if error}
	<Alert variant="destructive" class="mt-4">
		<TriangleAlert class="size-4" />
		<AlertTitle>Something went wrong</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{/if}

<div class="mt-6 space-y-6">
	<Card>
		<CardHeader>
			<CardTitle>Team profile</CardTitle>
			<CardDescription>
				Created {team?.created_at.slice(0, 10)} · {team?.members.length ?? 0}{' '}
				{(team?.members.length ?? 0) === 1 ? 'member' : 'members'}
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if renameError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{renameError}</AlertDescription>
				</Alert>
			{/if}
			<form
				onsubmit={(e) => {
					e.preventDefault();
					rename();
				}}
				class="space-y-3"
			>
				<div class="space-y-2">
					<Label for="team-name">Team name</Label>
					<div class="flex gap-2">
						<Input
							id="team-name"
							bind:value={renameName}
							class="flex-1"
							required
							maxlength={128}
							disabled={!canRename}
						/>
						{#if canRename}
							<Button type="submit" disabled={renaming}>
								<Save class="size-4" /> {renaming ? 'Saving…' : 'Save'}
							</Button>
						{/if}
					</div>
				</div>
			</form>
			{#if renamed}
				<Alert class="mt-3">
					<Check class="size-4" />
					<AlertDescription>Team renamed.</AlertDescription>
				</Alert>
			{/if}
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<CardTitle>Danger zone</CardTitle>
			<CardDescription>Actions that affect the whole Team.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-3">
			<div class="flex items-center justify-between gap-3 rounded-md border p-3">
				<div>
					<p class="text-sm font-medium">Leave Team</p>
					<p class="text-xs text-muted-foreground">Your Links stay with the Team.</p>
				</div>
				<Button variant="outline" disabled={confirmRequest !== null} onclick={leave}>
					<LogOut class="size-4" /> Leave
				</Button>
			</div>
			{#if isAdmin}
				<div class="flex items-center justify-between gap-3 rounded-md border border-destructive/40 p-3">
					<div>
						<p class="text-sm font-medium text-destructive">Delete Team</p>
						<p class="text-xs text-muted-foreground">Its Links revert to Personal for their Creators.</p>
					</div>
					<Button
						variant="destructive"
						disabled={confirmRequest !== null}
						onclick={removeTeam}
					>
						<Trash2 class="size-4" /> Delete Team
					</Button>
				</div>
			{/if}
		</CardContent>
	</Card>
</div>

<ConfirmDialog request={confirmRequest} onclose={() => (confirmRequest = null)} />
