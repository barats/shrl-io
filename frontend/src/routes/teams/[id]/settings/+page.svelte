<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import { api } from '$lib/api';
	import type { InviteCode, User } from '$lib/types';
	import ConfirmDialog, { type ConfirmRequest } from '$lib/components/ConfirmDialog.svelte';
	import SectionNav from '$lib/components/SectionNav.svelte';
	import { friendlyDate } from '$lib/utils';
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
		Check,
		Copy,
		KeyRound,
		LogOut,
		Save,
		Trash2,
		TriangleAlert,
		UserPlus
	} from '@lucide/svelte';

	const teamId = $derived(page.params.id ?? '');
	const team = $derived(page.data.team);
	const myRole = $derived(page.data.myRole);
	const isAdmin = $derived(page.data.user?.isAdmin ?? false);
	const isOwner = $derived(myRole === 'owner');
	const canRename = $derived(myRole === 'owner' || isAdmin);
	// myRole is null for admins viewing a Team they do not belong to.
	const canLeave = $derived(myRole !== null);
	const myUsername = $derived(page.data.user?.username ?? '');

	const sections = [
		{ id: 'profile', label: 'Profile' },
		{ id: 'members', label: 'Members' },
		{ id: 'danger-zone', label: 'Danger zone' }
	];

	let me = $state<User | null>(null);
	let error = $state('');

	// In-app confirm dialog for destructive actions (replaces native confirm()).
	let confirmRequest = $state<ConfirmRequest | null>(null);

	// Profile
	let renameName = $state('');
	let renaming = $state(false);
	let renameError = $state('');
	let renamed = $state(false);

	// Members
	let addUsername = $state('');
	let addingMember = $state(false);
	let memberError = $state('');

	// Invite codes
	let invites = $state<InviteCode[]>([]);
	let invitesLoading = $state(true);
	let invitesError = $state('');
	let copied = $state('');

	onMount(async () => {
		renameName = team?.name ?? '';
		try {
			me = await api.me();
		} catch (e) {
			error = (e as Error).message;
		}
		if (isOwner) loadInvites();
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

	async function loadInvites() {
		invitesLoading = true;
		invitesError = '';
		try {
			invites = await api.listInvites(teamId);
		} catch (e) {
			invitesError = (e as Error).message;
		} finally {
			invitesLoading = false;
		}
	}

	async function generateInvite() {
		invitesError = '';
		try {
			await api.createInvite(teamId);
			await loadInvites();
		} catch (e) {
			invitesError = (e as Error).message;
		}
	}

	function revokeInvite(code: string) {
		confirmRequest = {
			title: 'Revoke this invite code?',
			description: 'The code stops working immediately. This cannot be undone.',
			confirmLabel: 'Revoke',
			destructive: true,
			action: async () => {
				invitesError = '';
				try {
					await api.revokeInvite(teamId, code);
					await loadInvites();
				} catch (e) {
					invitesError = (e as Error).message;
				}
			}
		};
	}

	async function copyCode(code: string) {
		try {
			await navigator.clipboard.writeText(code);
			copied = code;
			setTimeout(() => (copied = ''), 1500);
		} catch {
			/* clipboard unavailable */
		}
	}

	async function addMember() {
		addingMember = true;
		memberError = '';
		try {
			await api.addTeamMember(teamId, addUsername.trim());
			addUsername = '';
			await invalidateAll();
		} catch (e) {
			memberError = (e as Error).message;
		} finally {
			addingMember = false;
		}
	}

	async function setRole(username: string, role: 'owner' | 'member') {
		memberError = '';
		try {
			await api.setTeamMemberRole(teamId, username, role);
			await invalidateAll();
		} catch (e) {
			memberError = (e as Error).message;
		}
	}

	function removeMember(member: { username: string }) {
		confirmRequest = {
			title: `Remove ${member.username}?`,
			description:
				'They lose read access to this Team. Links they created stay with the Team.',
			confirmLabel: 'Remove',
			destructive: true,
			action: async () => {
				memberError = '';
				try {
					await api.removeTeamMember(teamId, member.username);
					await invalidateAll();
				} catch (e) {
					memberError = (e as Error).message;
				}
			}
		};
	}

	function leave() {
		if (!me) return;
		const meName = me.username;
		confirmRequest = {
			title: 'Leave this Team?',
			description: 'Your Links stay with the Team. You can rejoin with a new invite code.',
			confirmLabel: 'Leave',
			action: async () => {
				try {
					await api.removeTeamMember(teamId, meName);
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
<p class="mt-1 text-sm text-muted-foreground">
	Team administration: profile, Members, and destructive actions.
</p>

{#if error}
	<Alert variant="destructive" class="mt-4">
		<TriangleAlert class="size-4" />
		<AlertTitle>Something went wrong</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{/if}

<div class="mt-6 grid gap-8 md:grid-cols-[200px_minmax(0,1fr)]">
	<SectionNav {sections} label="Team settings sections" />

	<div class="min-w-0 max-w-3xl space-y-6">
		<section id="profile" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Team profile</CardTitle>
					<CardDescription>
						Created {team ? friendlyDate(team.created_at) : ''} · {team?.members.length ?? 0}{' '}
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
		</section>

		<section id="members" class="scroll-mt-8 space-y-6">
			<Card>
				<CardHeader>
					<CardTitle class="flex items-center gap-2">
						Members
						<span class="text-sm font-normal text-muted-foreground">
							{team?.members.length ?? 0}
						</span>
					</CardTitle>
				</CardHeader>
				<CardContent>
					{#if memberError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertDescription>{memberError}</AlertDescription>
						</Alert>
					{/if}
					<ul class="divide-y">
						{#each team?.members ?? [] as member (member.username)}
							<li class="flex items-center gap-3 py-3">
								<span
									class="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary"
								>
									{member.username.slice(0, 1).toUpperCase()}
								</span>
								<span class="min-w-0 flex-1">
									<span class="flex items-center gap-2">
										<span class="truncate font-medium">{member.username}</span>
										{#if member.username === myUsername}
											<span class="text-xs text-muted-foreground">(you)</span>
										{/if}
									</span>
									{#if !member.joined_at.startsWith('0001')}
										<span class="text-xs text-muted-foreground">
											Joined {friendlyDate(member.joined_at)}
										</span>
									{/if}
								</span>
								<span class="flex shrink-0 items-center gap-1.5">
									{#if member.role === 'owner'}
										<Badge>Owner</Badge>
									{:else}
										<Badge variant="secondary">Member</Badge>
									{/if}
									{#if (isOwner || isAdmin) && member.username !== myUsername}
										{#if member.role === 'owner'}
											<Button
												variant="outline"
												size="sm"
												onclick={() => setRole(member.username, 'member')}
											>
												Demote
											</Button>
										{:else}
											<Button
												variant="outline"
												size="sm"
												onclick={() => setRole(member.username, 'owner')}
											>
												Promote
											</Button>
										{/if}
										<Button
											variant="ghost"
											size="icon-sm"
											title="Remove member"
											aria-label="Remove member"
											disabled={confirmRequest !== null}
											onclick={() => removeMember(member)}
										>
											<Trash2 class="size-4" />
										</Button>
									{/if}
								</span>
							</li>
						{/each}
					</ul>
					{#if isAdmin}
						<form
							onsubmit={(e) => {
								e.preventDefault();
								addMember();
							}}
							class="mt-4 flex gap-2"
						>
							<Input
								placeholder="Add a member by username…"
								bind:value={addUsername}
								class="flex-1"
								required
							/>
							<Button type="submit" disabled={addingMember}>
								<UserPlus class="size-4" /> Add
							</Button>
						</form>
					{/if}
				</CardContent>
			</Card>

			{#if isOwner}
				<Card>
					<CardHeader>
						<CardTitle>Invite codes</CardTitle>
						<CardDescription>
							Generate a single-use code and share it with someone who should join. Codes
							stop working once used or revoked.
						</CardDescription>
					</CardHeader>
					<CardContent>
						{#if invitesError}
							<Alert variant="destructive" class="mb-4">
								<TriangleAlert class="size-4" />
								<AlertDescription>{invitesError}</AlertDescription>
							</Alert>
						{/if}
						{#if invitesLoading}
							<Skeleton class="h-10 w-full" />
						{:else if invites.length === 0}
							<div class="flex items-center gap-3 py-2">
								<div
									class="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/50 text-muted-foreground"
								>
									<KeyRound class="size-4" />
								</div>
								<div>
									<p class="text-sm font-medium">No outstanding invite codes</p>
									<p class="text-sm text-muted-foreground">
										Generate one below to invite someone.
									</p>
								</div>
							</div>
						{:else}
							<ul class="divide-y">
								{#each invites as inv (inv.code)}
									<li class="flex items-center justify-between gap-2 py-2.5">
										<code
											class="rounded bg-muted px-2 py-1 text-sm font-semibold tracking-wide"
										>
											{inv.code}
										</code>
										<span class="flex shrink-0 gap-1.5">
											<Button variant="outline" size="sm" onclick={() => copyCode(inv.code)}>
												<Copy class="size-4" />
												{copied === inv.code ? 'Copied' : 'Copy'}
											</Button>
											<Button
												variant="ghost"
												size="icon-sm"
												title="Revoke code"
												aria-label="Revoke code"
												disabled={confirmRequest !== null}
												onclick={() => revokeInvite(inv.code)}
											>
												<Trash2 class="size-4" />
											</Button>
										</span>
									</li>
								{/each}
							</ul>
						{/if}
						<Button class="mt-4 w-full" onclick={generateInvite}>
							<KeyRound class="size-4" /> Generate invite code
						</Button>
					</CardContent>
				</Card>
			{/if}
		</section>

		<section id="danger-zone" class="scroll-mt-8">
			<Card>
				<CardHeader>
					<CardTitle>Danger zone</CardTitle>
					<CardDescription>Actions that affect the whole Team.</CardDescription>
				</CardHeader>
				<CardContent class="space-y-3">
					{#if canLeave}
						<div class="flex items-center justify-between gap-3 rounded-md border p-3">
							<div>
								<p class="text-sm font-medium">Leave Team</p>
								<p class="text-xs text-muted-foreground">Your Links stay with the Team.</p>
							</div>
							<Button
								variant="outline"
								disabled={confirmRequest !== null || !me}
								onclick={leave}
							>
								<LogOut class="size-4" /> Leave
							</Button>
						</div>
					{/if}
					{#if isAdmin}
						<div
							class="flex items-center justify-between gap-3 rounded-md border border-destructive/40 p-3"
						>
							<div>
								<p class="text-sm font-medium text-destructive">Delete Team</p>
								<p class="text-xs text-muted-foreground">
									Its Links revert to Personal for their Creators.
								</p>
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
		</section>
	</div>
</div>

<ConfirmDialog request={confirmRequest} onclose={() => (confirmRequest = null)} />
