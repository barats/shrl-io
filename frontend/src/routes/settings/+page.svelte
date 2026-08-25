<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { Team, User } from '$lib/types';
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
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import {
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TableRow
	} from '$lib/components/ui/table';
	import { KeyRound, Plus, Trash2, TriangleAlert, UserPlus, Users } from '@lucide/svelte';

	let me = $state<User | null>(null);

	// Hostnames
	let hostnames = $state<string[]>([]);
	let hLoading = $state(true);
	let hError = $state('');
	let newHostname = $state('');
	let addingHostname = $state(false);
	let hostnameError = $state('');

	// Users
	let users = $state<User[]>([]);
	let uLoading = $state(true);
	let uError = $state('');
	let newUsername = $state('');
	let newPassword = $state('');
	let newIsAdmin = $state(false);
	let creatingUser = $state(false);
	let userCreateError = $state('');
	let generatedPassword = $state('');
	let deleting = $state<number | null>(null);
	let userActionError = $state('');

	// Teams
	let teams = $state<Team[]>([]);
	let tLoading = $state(true);
	let tError = $state('');
	let newTeamName = $state('');
	let creatingTeam = $state(false);
	let teamCreateError = $state('');
	let deletingTeam = $state<number | null>(null);
	let teamActionError = $state('');

	onMount(async () => {
		try {
			me = await api.me();
		} catch {
			/* session is required; the layout guards it */
		}
		await Promise.all([loadHostnames(), loadUsers(), loadTeams()]);
	});

	async function loadHostnames() {
		hLoading = true;
		hError = '';
		try {
			hostnames = await api.hostnames();
		} catch (e) {
			hError = (e as Error).message;
		} finally {
			hLoading = false;
		}
	}

	async function addHostname() {
		addingHostname = true;
		hostnameError = '';
		try {
			await api.createHostname(newHostname.trim());
			newHostname = '';
			await loadHostnames();
		} catch (e) {
			hostnameError = (e as Error).message;
		} finally {
			addingHostname = false;
		}
	}

	async function removeHostname(name: string) {
		if (!window.confirm(`Remove ${name} from the registry? Existing Links on it keep serving.`)) return;
		hostnameError = '';
		try {
			await api.deleteHostname(name);
			await loadHostnames();
		} catch (e) {
			hostnameError = (e as Error).message;
		}
	}

	async function loadUsers() {
		uLoading = true;
		uError = '';
		try {
			users = await api.listUsers();
		} catch (e) {
			uError = (e as Error).message;
		} finally {
			uLoading = false;
		}
	}

	async function createUser() {
		creatingUser = true;
		userCreateError = '';
		generatedPassword = '';
		try {
			const res = await api.createUser({
				username: newUsername,
				password: newPassword || undefined,
				is_admin: newIsAdmin
			});
			if (!newPassword) generatedPassword = res.password;
			newUsername = '';
			newPassword = '';
			newIsAdmin = false;
			await loadUsers();
		} catch (e) {
			userCreateError = (e as Error).message;
		} finally {
			creatingUser = false;
		}
	}

	async function deleteUser(user: User) {
		if (!window.confirm(`Delete user ${user.username}? Their Personal Links and memberships are removed.`)) return;
		deleting = user.id;
		userActionError = '';
		try {
			await api.deleteUser(user.id);
			await loadUsers();
		} catch (e) {
			userActionError = (e as Error).message;
		} finally {
			deleting = null;
		}
	}

	async function loadTeams() {
		tLoading = true;
		tError = '';
		try {
			teams = await api.listTeams();
		} catch (e) {
			tError = (e as Error).message;
		} finally {
			tLoading = false;
		}
	}

	async function createTeam() {
		creatingTeam = true;
		teamCreateError = '';
		try {
			await api.createTeam(newTeamName);
			newTeamName = '';
			await loadTeams();
		} catch (e) {
			teamCreateError = (e as Error).message;
		} finally {
			creatingTeam = false;
		}
	}

	async function deleteTeam(team: Team) {
		if (!window.confirm(`Delete team ${team.name}? Its Links revert to Personal.`)) return;
		deletingTeam = team.id;
		teamActionError = '';
		try {
			await api.deleteTeam(team.id);
			await loadTeams();
		} catch (e) {
			teamActionError = (e as Error).message;
		} finally {
			deletingTeam = null;
		}
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
<p class="mt-1 text-sm text-muted-foreground">
	Instance administration: Hostnames, accounts, and Teams.
</p>

<div class="mt-6 space-y-6">
	<Card>
		<CardHeader>
			<CardTitle>Hostnames</CardTitle>
			<CardDescription>
				Users select from the Registry when creating a Link; a Hostname is never typed.
				Removing one only unregisters it — existing Links keep serving.
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if hostnameError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{hostnameError}</AlertDescription>
				</Alert>
			{/if}
			{#if hLoading}
				<div class="space-y-3">
					{#each [0, 1, 2] as i (i)}
						<Skeleton class="h-10 w-full" />
					{/each}
				</div>
			{:else}
				<ul class="divide-y">
					{#each hostnames as name (name)}
						<li class="flex items-center justify-between gap-2 py-2.5">
							<code class="text-sm font-medium">{name}</code>
							<Button
								variant="ghost"
								size="sm"
								title="Remove hostname"
								onclick={() => removeHostname(name)}
							>
								<Trash2 class="size-4" />
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
			<form
				onsubmit={(e) => {
					e.preventDefault();
					addHostname();
				}}
				class="mt-4 flex gap-2"
			>
				<Input
					placeholder="example.com"
					bind:value={newHostname}
					class="flex-1"
					aria-label="New hostname"
					required
				/>
				<Button type="submit" disabled={addingHostname}>
					<Plus class="size-4" /> Register
				</Button>
			</form>
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<CardTitle>Accounts</CardTitle>
			<CardDescription>
				Users sign in and manage their own Links. Deleting a user removes their Personal
				Links and memberships; Team Links they created stay with the Team.
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if userActionError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{userActionError}</AlertDescription>
				</Alert>
			{/if}
			{#if uError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertTitle>Could not load accounts</AlertTitle>
					<AlertDescription>{uError}</AlertDescription>
				</Alert>
			{:else if uLoading}
				<div class="space-y-3">
					{#each [0, 1, 2] as i (i)}
						<Skeleton class="h-10 w-full" />
					{/each}
				</div>
			{:else}
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Username</TableHead>
							<TableHead class="w-24">Role</TableHead>
							<TableHead class="w-36">Created</TableHead>
							<TableHead class="w-16"></TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{#each users as user (user.id)}
							<TableRow>
								<TableCell class="font-medium">{user.username}</TableCell>
								<TableCell>
									{#if user.is_admin}
										<Badge>Admin</Badge>
									{:else}
										<Badge variant="secondary">User</Badge>
									{/if}
								</TableCell>
								<TableCell class="text-muted-foreground">
									{user.created_at.slice(0, 10)}
								</TableCell>
								<TableCell>
									<Button
										variant="ghost"
										size="icon-sm"
										title={user.id === me?.id ? 'You cannot delete your own account' : 'Delete user'}
										disabled={user.id === me?.id || deleting === user.id}
										onclick={() => deleteUser(user)}
									>
										<Trash2 class="size-4" />
									</Button>
								</TableCell>
							</TableRow>
						{/each}
					</TableBody>
				</Table>
			{/if}

			<div class="mt-6 border-t pt-6">
				<h2 class="text-sm font-medium">Create an account</h2>
				<p class="mb-4 mt-1 text-sm text-muted-foreground">
					Leave the password blank to generate one, shown only once.
				</p>
				{#if userCreateError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not create account</AlertTitle>
						<AlertDescription>{userCreateError}</AlertDescription>
					</Alert>
				{/if}
				{#if generatedPassword}
					<Alert class="mb-4">
						<KeyRound class="size-4" />
						<AlertTitle>Account created — password shown once</AlertTitle>
						<AlertDescription>
							Share this password with the user:
							<span class="mt-2 block font-mono font-semibold">{generatedPassword}</span>
						</AlertDescription>
					</Alert>
				{/if}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						createUser();
					}}
					class="flex flex-wrap items-end gap-4"
				>
					<div class="space-y-2">
						<Label for="new-user-username">Username</Label>
						<Input id="new-user-username" bind:value={newUsername} placeholder="jane" required />
					</div>
					<div class="space-y-2">
						<Label for="new-user-password">Password (optional)</Label>
						<Input
							id="new-user-password"
							type="password"
							bind:value={newPassword}
							placeholder="blank = generate one"
						/>
					</div>
					<label class="flex h-8 items-center gap-2 text-sm">
						<Checkbox bind:checked={newIsAdmin} />
						Admin
					</label>
					<Button type="submit" disabled={creatingUser}>
						<UserPlus class="size-4" /> Create account
					</Button>
				</form>
			</div>
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<CardTitle>Teams</CardTitle>
			<CardDescription>
				Teams own Links; their members read them. Deleting a Team reverts its Links to Personal.
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if teamActionError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{teamActionError}</AlertDescription>
				</Alert>
			{/if}
			{#if tError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertTitle>Could not load Teams</AlertTitle>
					<AlertDescription>{tError}</AlertDescription>
				</Alert>
			{:else if tLoading}
				<div class="space-y-3">
					{#each [0, 1, 2] as i (i)}
						<Skeleton class="h-10 w-full" />
					{/each}
				</div>
			{:else if teams.length === 0}
				<p class="py-4 text-sm text-muted-foreground">No Teams yet.</p>
			{:else}
				<ul class="divide-y">
					{#each teams as team (team.id)}
						<li class="flex items-center justify-between gap-2 py-2.5">
							<a
								href={`/teams/${team.id}`}
								class="flex min-w-0 items-center gap-2 font-medium hover:underline"
							>
								<Users class="size-4 shrink-0 text-muted-foreground" />
								<span class="truncate">{team.name}</span>
							</a>
							<span class="flex shrink-0 items-center gap-2">
								{#if team.role === 'owner'}
									<Badge>Owner</Badge>
								{:else if team.role === 'member'}
									<Badge variant="secondary">Member</Badge>
								{:else}
									<Badge variant="outline">Not a member</Badge>
								{/if}
								<Button
									variant="ghost"
									size="sm"
									title="Delete team"
									disabled={deletingTeam === team.id}
									onclick={() => deleteTeam(team)}
								>
									<Trash2 class="size-4" />
								</Button>
							</span>
						</li>
					{/each}
				</ul>
			{/if}
			<form
				onsubmit={(e) => {
					e.preventDefault();
					createTeam();
				}}
				class="mt-4 flex gap-2"
			>
				<Input
					placeholder="New team name"
					bind:value={newTeamName}
					class="flex-1"
					aria-label="New team name"
					required
				/>
				<Button type="submit" disabled={creatingTeam}>
					<Plus class="size-4" /> Create
				</Button>
			</form>
		</CardContent>
	</Card>
</div>
