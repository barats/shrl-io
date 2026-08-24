<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { User } from '$lib/types';
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
	import { KeyRound, TriangleAlert, UserPlus } from '@lucide/svelte';

	let users = $state<User[]>([]);
	let loading = $state(true);
	let error = $state('');

	let newUsername = $state('');
	let newPassword = $state('');
	let newIsAdmin = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let generatedPassword = $state('');

	onMount(async () => {
		try {
			users = await api.listUsers();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function create() {
		creating = true;
		createError = '';
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
			users = await api.listUsers();
		} catch (e) {
			createError = (e as Error).message;
		} finally {
			creating = false;
		}
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">Users</h1>

<div class="mt-4 grid gap-6 lg:grid-cols-3">
	<div class="lg:col-span-2">
		<Card>
			<CardHeader>
				<CardTitle>Accounts</CardTitle>
				<CardDescription>Users can sign in and manage their own Links.</CardDescription>
			</CardHeader>
			<CardContent>
				{#if error}
					<Alert variant="destructive">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not load Users</AlertTitle>
						<AlertDescription>{error}</AlertDescription>
					</Alert>
				{:else if loading}
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
								</TableRow>
							{/each}
						</TableBody>
					</Table>
				{/if}
			</CardContent>
		</Card>
	</div>

	<div>
		<Card>
			<CardHeader>
				<CardTitle>Create an account</CardTitle>
				<CardDescription>Leave the password blank to generate one, shown only once.</CardDescription>
			</CardHeader>
			<CardContent>
				{#if createError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not create account</AlertTitle>
						<AlertDescription>{createError}</AlertDescription>
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
						create();
					}}
					class="space-y-4"
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
					<label class="flex items-center gap-2 text-sm">
						<Checkbox bind:checked={newIsAdmin} />
						Give this user admin privileges
					</label>
					<Button type="submit" class="w-full" disabled={creating}>
						{#if creating}
							Creating…
						{:else}
							<UserPlus class="size-4" /> Create account
						{/if}
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
</div>
