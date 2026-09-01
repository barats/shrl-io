<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { ApiKey, User } from '$lib/types';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
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
	import {
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TableRow
	} from '$lib/components/ui/table';
	import { KeyRound, Plus, Trash2, TriangleAlert } from '@lucide/svelte';

	let me = $state<User | null>(null);

	// Change password
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);
	let passwordError = $state('');
	let passwordSuccess = $state('');

	// API keys
	let keys = $state<ApiKey[]>([]);
	let kLoading = $state(true);
	let kError = $state('');
	let newKeyName = $state('');
	let creatingKey = $state(false);
	let keyError = $state('');
	let newSecret = $state('');
	let revoking = $state<number | null>(null);

	onMount(async () => {
		try {
			me = await api.me();
		} catch {
			/* session is required; the layout guards it */
		}
		if (me && !me.must_change_password) {
			await loadKeys();
		} else {
			kLoading = false;
		}
	});

	async function changePassword(event: SubmitEvent) {
		event.preventDefault();
		passwordError = '';
		passwordSuccess = '';
		if (newPassword !== confirmPassword) {
			passwordError = 'Passwords do not match';
			return;
		}
		changingPassword = true;
		try {
			await api.changePassword(currentPassword, newPassword);
			passwordSuccess = 'Password changed.';
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			me = await api.me();
			if (me && !me.must_change_password) {
				await loadKeys();
			}
		} catch (e) {
			passwordError = (e as Error).message;
		} finally {
			changingPassword = false;
		}
	}

	async function loadKeys() {
		kLoading = true;
		kError = '';
		try {
			keys = await api.listApiKeys();
		} catch (e) {
			kError = (e as Error).message;
		} finally {
			kLoading = false;
		}
	}

	async function createKey(event: SubmitEvent) {
		event.preventDefault();
		creatingKey = true;
		keyError = '';
		newSecret = '';
		try {
			const res = await api.createApiKey(newKeyName.trim());
			newSecret = res.secret;
			newKeyName = '';
			await loadKeys();
		} catch (e) {
			keyError = (e as Error).message;
		} finally {
			creatingKey = false;
		}
	}

	async function revokeKey(id: number) {
		if (!window.confirm('Revoke this API key? Anything using it stops working immediately.')) return;
		revoking = id;
		keyError = '';
		try {
			await api.revokeApiKey(id);
			await loadKeys();
		} catch (e) {
			keyError = (e as Error).message;
		} finally {
			revoking = null;
		}
	}
</script>

<svelte:head>
	<title>Account - shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Account</h1>
<p class="mt-1 text-sm text-muted-foreground">
	Your password and API keys for programmatic access.
</p>

<div class="mt-6 space-y-6">
	{#if me?.must_change_password}
		<Alert>
			<TriangleAlert class="size-4" />
			<AlertTitle>Password change required</AlertTitle>
			<AlertDescription>
				An admin reset your password. Set a new one below before using shrl.io.
			</AlertDescription>
		</Alert>
	{/if}

	<Card>
		<CardHeader>
			<CardTitle>Change password</CardTitle>
			<CardDescription>
				Changing your password signs out every other session and revokes all API keys.
				Only this session stays signed in.
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if passwordError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{passwordError}</AlertDescription>
				</Alert>
			{/if}
			{#if passwordSuccess}
				<Alert class="mb-4">
					<KeyRound class="size-4" />
					<AlertDescription>{passwordSuccess}</AlertDescription>
				</Alert>
			{/if}
			<form onsubmit={changePassword} class="space-y-4">
				<div class="space-y-2">
					<Label for="current-password">Current password</Label>
					<Input
						id="current-password"
						type="password"
						bind:value={currentPassword}
						autocomplete="current-password"
						required
					/>
				</div>
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-2">
						<Label for="new-password">New password</Label>
						<Input
							id="new-password"
							type="password"
							bind:value={newPassword}
							autocomplete="new-password"
							minlength={8}
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="confirm-password">Confirm new password</Label>
						<Input
							id="confirm-password"
							type="password"
							bind:value={confirmPassword}
							autocomplete="new-password"
							minlength={8}
							required
						/>
					</div>
				</div>
				<Button type="submit" disabled={changingPassword}>
					{changingPassword ? 'Saving…' : 'Change password'}
				</Button>
			</form>
		</CardContent>
	</Card>

	{#if me && !me.must_change_password}
		<Card>
			<CardHeader>
				<CardTitle>API keys</CardTitle>
				<CardDescription>
					Long-lived credentials for scripts and CI. They never expire, are revoked
					explicitly, and are shown only once at creation.
				</CardDescription>
			</CardHeader>
			<CardContent>
				{#if keyError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertDescription>{keyError}</AlertDescription>
					</Alert>
				{/if}
				{#if newSecret}
					<Alert class="mb-4">
						<KeyRound class="size-4" />
						<AlertTitle>Key created — shown once</AlertTitle>
						<AlertDescription>
							Copy it now; it will not be shown again:
							<span class="mt-2 block break-all font-mono font-semibold">{newSecret}</span>
						</AlertDescription>
					</Alert>
				{/if}
				{#if kError}
					<Alert variant="destructive" class="mb-4">
						<TriangleAlert class="size-4" />
						<AlertTitle>Could not load API keys</AlertTitle>
						<AlertDescription>{kError}</AlertDescription>
					</Alert>
				{:else if kLoading}
					<div class="space-y-3">
						{#each [0, 1, 2] as i (i)}
							<Skeleton class="h-10 w-full" />
						{/each}
					</div>
				{:else if keys.length === 0}
					<p class="py-4 text-sm text-muted-foreground">No API keys yet.</p>
				{:else}
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Name</TableHead>
								<TableHead class="w-36">Created</TableHead>
								<TableHead class="w-16"></TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{#each keys as key (key.id)}
								<TableRow>
									<TableCell class="font-medium">{key.name}</TableCell>
									<TableCell class="text-muted-foreground">
										{key.created_at.slice(0, 10)}
									</TableCell>
									<TableCell>
										<Button
											variant="ghost"
											size="icon-sm"
											title="Revoke key"
											disabled={revoking === key.id}
											onclick={() => revokeKey(key.id)}
										>
											<Trash2 class="size-4" />
										</Button>
									</TableCell>
								</TableRow>
							{/each}
						</TableBody>
					</Table>
				{/if}
				<form onsubmit={createKey} class="mt-4 flex gap-2">
					<Input
						placeholder="e.g. ci"
						bind:value={newKeyName}
						class="flex-1"
						aria-label="New key name"
						maxlength={64}
						required
					/>
					<Button type="submit" disabled={creatingKey}>
						<Plus class="size-4" /> Create key
					</Button>
				</form>
			</CardContent>
		</Card>
	{/if}
</div>
