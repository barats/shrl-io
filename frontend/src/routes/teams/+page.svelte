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
	import { ChevronRight, KeyRound, Plus, TriangleAlert, Users } from '@lucide/svelte';
	import { relativeDate } from '$lib/utils';

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

	// GET /teams returns every Team to an Admin, with an empty role for
	// Teams they do not belong to; everyone else sees only memberships.
	let myTeams = $derived(teams.filter((t) => t.role !== ''));
	let otherTeams = $derived(teams.filter((t) => t.role === ''));

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
	<title>Teams - shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Teams</h1>
<p class="mt-1 text-sm text-muted-foreground">
	Teams own Links. Members read them; their Creators and Team Owners manage them.
</p>

<div class="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_260px]">
	<div class="min-w-0 space-y-6">
		{#if error}
			<Alert variant="destructive">
				<TriangleAlert class="size-4" />
				<AlertTitle>Failed to load Teams</AlertTitle>
				<AlertDescription>{error}</AlertDescription>
			</Alert>
		{:else if loading}
			<Card>
				<CardHeader>
					<CardTitle>Your Teams</CardTitle>
				</CardHeader>
				<CardContent>
					<div class="space-y-3">
						{#each [0, 1, 2] as i (i)}
							<Skeleton class="h-12 w-full" />
						{/each}
					</div>
				</CardContent>
			</Card>
		{:else if teams.length === 0}
			<Card>
				<CardHeader>
					<CardTitle>Your Teams</CardTitle>
				</CardHeader>
				<CardContent>
					<div class="flex items-center gap-3 py-2">
						<div
							class="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/50 text-muted-foreground"
						>
							<Users class="size-4" />
						</div>
						<div>
							<p class="text-sm font-medium">No Teams yet</p>
							<p class="text-sm text-muted-foreground">
								Join one with an invite code below{#if isAdmin}, or create one{/if}.
							</p>
						</div>
					</div>
				</CardContent>
			</Card>
		{:else}
			{#if myTeams.length > 0}
				<Card>
					<CardHeader>
						<CardTitle>Your Teams</CardTitle>
						<CardDescription>
							Team Members read a Team's Links; its Creator and Team Owners manage them.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<ul class="divide-y">
							{#each myTeams as team (team.id)}
								<li>
									<a
										href={`/teams/${team.id}`}
										class="flex items-center gap-3 py-3 transition-colors hover:bg-muted/50"
									>
										<span
											class="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary"
										>
											{team.name.slice(0, 1).toUpperCase()}
										</span>
										<span class="min-w-0 flex-1 truncate font-medium">{team.name}</span>
										<span class="flex shrink-0 items-center gap-3">
											{#if team.role === 'owner'}
												<Badge>Owner</Badge>
											{:else}
												<Badge variant="secondary">Member</Badge>
											{/if}
											<span
												class="hidden text-xs text-muted-foreground sm:inline"
												title={team.created_at.slice(0, 10)}
											>
												{relativeDate(team.created_at)}
											</span>
											<ChevronRight class="size-4 text-muted-foreground" />
										</span>
									</a>
								</li>
							{/each}
						</ul>
					</CardContent>
				</Card>
			{/if}

			{#if isAdmin && otherTeams.length > 0}
				<Card>
					<CardHeader>
						<CardTitle>Other Teams</CardTitle>
						<CardDescription>
							Teams on this instance you do not belong to. Manage each one from its
							Settings page.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<ul class="divide-y">
							{#each otherTeams as team (team.id)}
								<li>
									<a
										href={`/teams/${team.id}`}
										class="flex items-center gap-3 py-3 transition-colors hover:bg-muted/50"
									>
										<span
											class="flex size-9 shrink-0 items-center justify-center rounded-full border bg-muted/50 text-sm font-semibold text-muted-foreground"
										>
											{team.name.slice(0, 1).toUpperCase()}
										</span>
										<span class="min-w-0 flex-1 truncate font-medium">{team.name}</span>
										<span class="flex shrink-0 items-center gap-3">
											<span
												class="hidden text-xs text-muted-foreground sm:inline"
												title={team.created_at.slice(0, 10)}
											>
												{relativeDate(team.created_at)}
											</span>
											<ChevronRight class="size-4 text-muted-foreground" />
										</span>
									</a>
								</li>
							{/each}
						</ul>
					</CardContent>
				</Card>
			{/if}
		{/if}
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
						<Input
							id="join-code"
							bind:value={joinCode}
							placeholder="ABC12345"
							required
							oninput={() => (joinError = '')}
						/>
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
