<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { User } from '$lib/types';
	import ConfirmDialog, { type ConfirmRequest } from '$lib/components/ConfirmDialog.svelte';
	import SectionNav from '$lib/components/SectionNav.svelte';
	import { relativeDate } from '$lib/utils';
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
	import {
		Copy,
		Globe,
		KeyRound,
		Plus,
		Trash2,
		TriangleAlert,
		UserPlus
	} from '@lucide/svelte';

	let me = $state<User | null>(null);

	// Base URLs
	let baseURLs = $state<string[]>([]);
	let hLoading = $state(true);
	let hError = $state('');
	let newBaseURL = $state('');
	let addingBaseURL = $state(false);
	let baseURLError = $state('');

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
	let resetResult = $state('');
	let resetFor = $state('');
	let userActionError = $state('');

	// Code length
	let codeLength = $state(6);
	let cLoading = $state(true);
	let cError = $state('');
	let savingCodeLength = $state(false);
	let codeLengthSaved = $state(false);
	let codeLengthError = $state('');

	// In-app confirm dialog for destructive actions (replaces native confirm()).
	let confirmRequest = $state<ConfirmRequest | null>(null);

	const sections = [
		{ id: 'code-generation', label: 'Code generation' },
		{ id: 'base-urls', label: 'Base URLs' },
		{ id: 'users', label: 'Users' }
	];
	let secretCopied = $state('');

	onMount(async () => {
		try {
			me = await api.me();
		} catch {
			/* session is required; the layout guards it */
		}
		await Promise.all([loadCodeLength(), loadBaseURLs(), loadUsers()]);
	});

	async function loadCodeLength() {
		cLoading = true;
		cError = '';
		try {
			const s = await api.getSettings();
			codeLength = s.code_length;
		} catch (e) {
			cError = (e as Error).message;
		} finally {
			cLoading = false;
		}
	}

	async function saveCodeLength(event: SubmitEvent) {
		event.preventDefault();
		savingCodeLength = true;
		codeLengthError = '';
		codeLengthSaved = false;
		try {
			const s = await api.updateCodeLength(codeLength);
			codeLength = s.code_length;
			codeLengthSaved = true;
		} catch (e) {
			codeLengthError = (e as Error).message;
		} finally {
			savingCodeLength = false;
		}
	}

	async function loadBaseURLs() {
		hLoading = true;
		hError = '';
		try {
			baseURLs = await api.baseURLs();
		} catch (e) {
			hError = (e as Error).message;
		} finally {
			hLoading = false;
		}
	}

	async function addBaseURL() {
		addingBaseURL = true;
		baseURLError = '';
		try {
			await api.createBaseURL(newBaseURL.trim());
			newBaseURL = '';
			await loadBaseURLs();
		} catch (e) {
			baseURLError = (e as Error).message;
		} finally {
			addingBaseURL = false;
		}
	}

	function removeBaseURL(url: string) {
		confirmRequest = {
			title: 'Remove this Base URL?',
			description: `${url} leaves the Registry. Existing Links on it keep serving.`,
			confirmLabel: 'Remove',
			destructive: true,
			action: async () => {
				baseURLError = '';
				try {
					await api.deleteBaseURL(url);
					await loadBaseURLs();
				} catch (e) {
					baseURLError = (e as Error).message;
				}
			}
		};
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

	function deleteUser(user: User) {
		confirmRequest = {
			title: `Delete ${user.username}?`,
			description: 'Their Personal Links and memberships are removed. This cannot be undone.',
			confirmLabel: 'Delete',
			destructive: true,
			action: async () => {
				userActionError = '';
				try {
					await api.deleteUser(user.id);
					await loadUsers();
				} catch (e) {
					userActionError = (e as Error).message;
				}
			}
		};
	}

	function resetPassword(user: User) {
		confirmRequest = {
			title: `Reset ${user.username}'s password?`,
			description:
				'Their current sign-ins and API keys are revoked. You will get a temporary password to share.',
			confirmLabel: 'Reset password',
			destructive: true,
			action: async () => {
				userActionError = '';
				resetResult = '';
				try {
					const res = await api.resetUserPassword(user.id);
					resetResult = res.password;
					resetFor = user.username;
				} catch (e) {
					userActionError = (e as Error).message;
				}
			}
		};
	}

	async function copySecret(secret: string) {
		try {
			await navigator.clipboard.writeText(secret);
			secretCopied = secret;
			setTimeout(() => (secretCopied = ''), 2000);
		} catch {
			/* clipboard unavailable */
		}
	}
</script>

<svelte:head>
	<title>Settings - shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
<p class="mt-1 text-sm text-muted-foreground">
	Instance administration: Code length, Base URLs, and users. Teams live on the
	Teams page.
</p>

<div class="mt-6 grid gap-8 md:grid-cols-[200px_minmax(0,1fr)]">
	<SectionNav {sections} label="Settings sections" />

	<div class="min-w-0 max-w-3xl space-y-6">
		<section id="code-generation" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Code generation</CardTitle>
					<CardDescription>
						The exact length of every auto-generated Code (e.g. 6 → <code>abc123</code>).
						Applies to newly created Links; existing Links keep serving.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{#if cError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertTitle>Could not load settings</AlertTitle>
							<AlertDescription>{cError}</AlertDescription>
						</Alert>
					{/if}
					{#if codeLengthError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertDescription>{codeLengthError}</AlertDescription>
						</Alert>
					{/if}
					{#if codeLengthSaved}
						<Alert class="mb-4">
							<KeyRound class="size-4" />
							<AlertDescription>Code length saved. New Links will use it.</AlertDescription>
						</Alert>
					{/if}
					<form onsubmit={saveCodeLength} class="flex flex-wrap items-end gap-4">
						<div class="space-y-2">
							<Label for="code-length">Code length</Label>
							<Input
								id="code-length"
								type="number"
								min={4}
								max={12}
								step={1}
								bind:value={codeLength}
								class="w-28"
								disabled={cLoading}
								required
								aria-describedby="code-length-help"
							/>
							<p id="code-length-help" class="text-xs text-muted-foreground">
								Between 4 and 12 characters.
							</p>
						</div>
						<Button type="submit" disabled={savingCodeLength || cLoading}>
							{savingCodeLength ? 'Saving…' : 'Save'}
						</Button>
					</form>
				</CardContent>
			</Card>
		</section>

		<section id="base-urls" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Base URLs</CardTitle>
					<CardDescription>
						The public URL prefix under which Links are served (scheme, host, optional
						port and path). Users select from the Registry when creating a Link.
						Removing one only unregisters it. Existing Links keep serving.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{#if baseURLError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertDescription>{baseURLError}</AlertDescription>
						</Alert>
					{/if}
					{#if hLoading}
						<div class="space-y-3">
							{#each [0, 1, 2] as i (i)}
								<Skeleton class="h-10 w-full" />
							{/each}
						</div>
					{:else if baseURLs.length === 0}
						<div class="flex items-center gap-3 py-2">
							<div
								class="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/50 text-muted-foreground"
							>
								<Globe class="size-4" />
							</div>
							<div>
								<p class="text-sm font-medium">No Base URLs registered</p>
								<p class="text-sm text-muted-foreground">
									Register one below so Users can create Links.
								</p>
							</div>
						</div>
					{:else}
						<ul class="divide-y">
							{#each baseURLs as url (url)}
								<li class="flex items-center justify-between gap-2 py-2.5">
									<code class="text-sm font-medium">{url}</code>
									<Button
										variant="ghost"
										size="sm"
										title="Remove base URL"
										aria-label="Remove base URL"
										disabled={confirmRequest !== null}
										onclick={() => removeBaseURL(url)}
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
							addBaseURL();
						}}
						class="mt-4 flex gap-2"
					>
						<Input
							placeholder="https://example.com"
							bind:value={newBaseURL}
							class="flex-1"
							name="new-base-url"
							aria-label="New base URL"
							required
						/>
						<Button type="submit" disabled={addingBaseURL}>
							<Plus class="size-4" /> Register
						</Button>
					</form>
				</CardContent>
			</Card>
		</section>

		<section id="users" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Users</CardTitle>
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
					{#if resetResult}
						<div class="mb-4 rounded-lg border bg-muted/50 p-4">
							<div class="flex items-start justify-between gap-3">
								<div class="flex items-start gap-3">
									<div
										class="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
									>
										<KeyRound class="size-4" />
									</div>
									<div>
										<p class="text-sm font-medium">Password reset for {resetFor}</p>
										<p class="mt-0.5 text-sm text-muted-foreground">
											This temporary password is shown once. Share it with the user; they
											are asked to change it on next sign-in.
										</p>
									</div>
								</div>
								<Button
									type="button"
									variant="outline"
									size="sm"
									onclick={() => copySecret(resetResult)}
								>
									<Copy class="size-4" /> {secretCopied === resetResult ? 'Copied!' : 'Copy'}
								</Button>
							</div>
							<p
								class="mt-3 break-all rounded-md border bg-background px-3 py-2 font-mono text-sm font-semibold"
							>
								{resetResult}
							</p>
						</div>
					{/if}
					{#if uError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertTitle>Could not load users</AlertTitle>
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
											<span title={user.created_at.slice(0, 10)}>
												{relativeDate(user.created_at)}
											</span>
										</TableCell>
										<TableCell>
											<span class="flex items-center justify-end gap-1">
												<Button
													variant="ghost"
													size="icon-sm"
													title={
														user.id === me?.id
															? 'You cannot reset your own password here. Use the Profile page.'
															: 'Reset password'
													}
													aria-label="Reset password"
													disabled={user.id === me?.id || confirmRequest !== null}
													onclick={() => resetPassword(user)}
												>
													<KeyRound class="size-4" />
												</Button>
												<Button
													variant="ghost"
													size="icon-sm"
													title={user.id === me?.id
														? 'You cannot delete your own account'
														: 'Delete user'}
													aria-label="Delete user"
													disabled={user.id === me?.id || confirmRequest !== null}
													onclick={() => deleteUser(user)}
												>
													<Trash2 class="size-4" />
												</Button>
											</span>
										</TableCell>
									</TableRow>
								{/each}
							</TableBody>
						</Table>
					{/if}
				</CardContent>
			</Card>
		</section>

		<section id="create-user" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Create user</CardTitle>
					<CardDescription>
						Leave the password blank to generate one, shown only once.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{#if userCreateError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertTitle>Could not create user</AlertTitle>
							<AlertDescription>{userCreateError}</AlertDescription>
						</Alert>
					{/if}
					{#if generatedPassword}
						<div class="mb-4 rounded-lg border bg-muted/50 p-4">
							<div class="flex items-start justify-between gap-3">
								<div class="flex items-start gap-3">
									<div
										class="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
									>
										<KeyRound class="size-4" />
									</div>
									<div>
										<p class="text-sm font-medium">User created</p>
										<p class="mt-0.5 text-sm text-muted-foreground">
											This password is shown once. Share it with the user.
										</p>
									</div>
								</div>
								<Button
									type="button"
									variant="outline"
									size="sm"
									onclick={() => copySecret(generatedPassword)}
								>
									<Copy class="size-4" />
									{secretCopied === generatedPassword ? 'Copied!' : 'Copy'}
								</Button>
							</div>
							<p
								class="mt-3 break-all rounded-md border bg-background px-3 py-2 font-mono text-sm font-semibold"
							>
								{generatedPassword}
							</p>
						</div>
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
							<UserPlus class="size-4" /> Create user
						</Button>
					</form>
				</CardContent>
			</Card>
		</section>

	</div>
</div>

<ConfirmDialog request={confirmRequest} onclose={() => (confirmRequest = null)} />
