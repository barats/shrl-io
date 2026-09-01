<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { ApiKey, User } from '$lib/types';
	import ConfirmDialog, { type ConfirmRequest } from '$lib/components/ConfirmDialog.svelte';
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
	import {
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TableRow
	} from '$lib/components/ui/table';
	import { Copy, KeyRound, Plus, Trash2, TriangleAlert } from '@lucide/svelte';

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
	let confirmRequest = $state<ConfirmRequest | null>(null);
	let secretCopied = $state(false);

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

	function revokeKey(id: number) {
		confirmRequest = {
			title: 'Revoke this API key?',
			description: 'Anything using it stops working immediately. This cannot be undone.',
			confirmLabel: 'Revoke',
			destructive: true,
			action: async () => {
				keyError = '';
				try {
					await api.revokeApiKey(id);
					await loadKeys();
				} catch (e) {
					keyError = (e as Error).message;
				}
			}
		};
	}

	async function copySecret() {
		try {
			await navigator.clipboard.writeText(newSecret);
			secretCopied = true;
			setTimeout(() => (secretCopied = false), 2000);
		} catch {
			/* clipboard unavailable */
		}
	}
</script>

<svelte:head>
	<title>Account - shrl.io</title>
</svelte:head>

<div class="max-w-3xl">
	<h1 class="text-2xl font-semibold tracking-tight">Account</h1>
	<p class="mt-1 text-sm text-muted-foreground">
		Your password and API keys for programmatic access.
	</p>

	{#if me}
		<div class="mt-6 flex items-center gap-4">
			<div
				class="flex size-12 shrink-0 items-center justify-center rounded-full bg-primary/10 text-lg font-semibold text-primary"
			>
				{me.username.slice(0, 1).toUpperCase()}
			</div>
			<div class="min-w-0">
				<div class="flex items-center gap-2">
					<p class="truncate text-lg font-semibold leading-tight">{me.username}</p>
					{#if me.is_admin}
						<Badge>Admin</Badge>
					{/if}
				</div>
				<p class="text-sm text-muted-foreground">Member since {me.created_at.slice(0, 10)}</p>
			</div>
		</div>
	{/if}

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
				<form onsubmit={changePassword} class="max-w-sm space-y-4">
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
					<div class="space-y-2">
						<Label for="new-password">New password</Label>
						<Input
							id="new-password"
							type="password"
							bind:value={newPassword}
							autocomplete="new-password"
							minlength={8}
							required
							aria-describedby="new-password-help"
						/>
						<p id="new-password-help" class="text-xs text-muted-foreground">
							At least 8 characters.
						</p>
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
						<div class="mb-4 rounded-lg border bg-muted/50 p-4">
							<div class="flex items-start justify-between gap-3">
								<div class="flex items-start gap-3">
									<div
										class="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
									>
										<KeyRound class="size-4" />
									</div>
									<div>
										<p class="text-sm font-medium">API key created</p>
										<p class="mt-0.5 text-sm text-muted-foreground">
											This secret is shown once. Copy it now; it will not be shown again.
										</p>
									</div>
								</div>
								<Button type="button" variant="outline" size="sm" onclick={copySecret}>
									<Copy class="size-4" /> {secretCopied ? 'Copied!' : 'Copy'}
								</Button>
							</div>
							<p
								class="mt-3 break-all rounded-md border bg-background px-3 py-2 font-mono text-sm font-semibold"
							>
								{newSecret}
							</p>
						</div>
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
						<div class="flex items-center gap-3 py-2">
							<div
								class="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/50 text-muted-foreground"
							>
								<KeyRound class="size-4" />
							</div>
							<div>
								<p class="text-sm font-medium">No API keys yet</p>
								<p class="text-sm text-muted-foreground">
									Create one below for your scripts and CI.
								</p>
							</div>
						</div>
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
												disabled={confirmRequest !== null}
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
</div>

<ConfirmDialog request={confirmRequest} onclose={() => (confirmRequest = null)} />
