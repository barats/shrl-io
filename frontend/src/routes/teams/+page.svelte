<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import type { Team } from '$lib/types';
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
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { KeyRound, Plus, TriangleAlert, Users } from '@lucide/svelte';

	const isAdmin = $derived(page.data.user?.isAdmin ?? false);

	let teams = $state<Team[]>([]);
	let loading = $state(true);
	let error = $state('');

	let createName = $state('');
	let creating = $state(false);
	let createError = $state('');

	let joinCode = $state('');
	let joining = $state(false);
	let joinError = $state('');

	onMount(async () => {
		try {
			teams = await api.listTeams();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function createTeam() {
		creating = true;
		createError = '';
		try {
			await api.createTeam(createName);
			createName = '';
			teams = await api.listTeams();
		} catch (e) {
			createError = (e as Error).message;
		} finally {
			creating = false;
		}
	}

	async function join() {
		joining = true;
		joinError = '';
		try {
			const team = await api.joinTeam(joinCode.trim());
			await goto(`/teams/${team.id}`);
		} catch (e) {
			joinError = (e as Error).message;
		} finally {
			joining = false;
		}
	}
</script>

<svelte:head>
	<title>Teams — shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Teams</h1>

<div class="mt-4 grid gap-6 lg:grid-cols-3">
	<div class="lg:col-span-2">
		<Card>
			<CardHeader>
				<CardTitle>Your Teams</CardTitle>
				<CardDescription>
					Team Members read a Team's Links; its Creator and Team Owners manage them.
				</CardDescription>
			</CardHeader>
			<CardContent>
				{#if error}
					<Alert variant="destructive">
						<TriangleAlert class="size-4" />
						<AlertTitle>Failed to load Teams</AlertTitle>
						<AlertDescription>{error}</AlertDescription>
					</Alert>
				{:else if loading}
					<div class="space-y-3">
						{#each [0, 1, 2] as i (i)}
							<Skeleton class="h-12 w-full" />
						{/each}
					</div>
				{:else if teams.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">
						You're not in any Teams yet. Join one with an invite code, or ask an Admin to create it.
					</p>
				{:else}
					<ul class="divide-y">
						{#each teams as team (team.id)}
							<li>
								<a
									href={`/teams/${team.id}`}
									class="flex items-center justify-between gap-2 py-3 hover:bg-muted/50"
								>
									<span class="flex min-w-0 items-center gap-2 font-medium">
										<Users class="size-4 shrink-0 text-muted-foreground" />
										<span class="truncate">{team.name}</span>
									</span>
									<span class="flex shrink-0 items-center gap-2">
										{#if team.role === 'owner'}
											<Badge>Owner</Badge>
										{:else if team.role === 'member'}
											<Badge variant="secondary">Member</Badge>
										{:else}
											<Badge variant="outline">Not a member</Badge>
										{/if}
										<span class="text-xs text-muted-foreground">{team.created_at.slice(0, 10)}</span>
									</span>
								</a>
							</li>
						{/each}
					</ul>
				{/if}
			</CardContent>
		</Card>
	</div>

	<div class="space-y-6">
		{#if isAdmin}
			<Card>
				<CardHeader>
					<CardTitle>Create a Team</CardTitle>
					<CardDescription>You become its first Team Owner.</CardDescription>
				</CardHeader>
				<CardContent>
					{#if createError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertTitle>Could not create Team</AlertTitle>
							<AlertDescription>{createError}</AlertDescription>
						</Alert>
					{/if}
					<form
						onsubmit={(e) => {
							e.preventDefault();
							createTeam();
						}}
						class="space-y-3"
					>
						<div class="space-y-2">
							<Label for="team-name">Name</Label>
							<Input id="team-name" bind:value={createName} placeholder="growth" required />
						</div>
						<Button type="submit" class="w-full" disabled={creating}>
							{#if creating}
								Creating…
							{:else}
								<Plus class="size-4" /> Create Team
							{/if}
						</Button>
					</form>
				</CardContent>
			</Card>
		{/if}

		<Card>
			<CardHeader>
				<CardTitle>Join a Team</CardTitle>
				<CardDescription>Enter the invite code a Team Owner shared with you.</CardDescription>
			</CardHeader>
			<CardContent>
				{#if joinError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not join</AlertTitle>
						<AlertDescription>{joinError}</AlertDescription>
					</Alert>
				{/if}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						join();
					}}
					class="space-y-3"
				>
					<div class="space-y-2">
						<Label for="join-code">Invite code</Label>
						<Input id="join-code" bind:value={joinCode} placeholder="ABC12345" required />
					</div>
					<Button type="submit" class="w-full" disabled={joining}>
						{#if joining}
							Joining…
						{:else}
							<KeyRound class="size-4" /> Join Team
						{/if}
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
</div>
