<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { invalidateAll } from '$app/navigation';
	import { api } from '$lib/api';
	import type { InviteCode, TeamRole } from '$lib/types';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Copy, KeyRound, Trash2, TriangleAlert, UserPlus } from '@lucide/svelte';

	const team = $derived(page.data.team);
	const myRole = $derived(page.data.myRole);
	const isAdmin = $derived(page.data.user?.isAdmin ?? false);
	const isOwner = $derived(myRole === 'owner');
	const canManage = $derived(isOwner || isAdmin);
	const myUsername = $derived(page.data.user?.username ?? '');

	let invites = $state<InviteCode[]>([]);
	let invitesLoading = $state(true);
	let invitesError = $state('');
	let copied = $state('');

	let addUsername = $state('');
	let addingMember = $state(false);
	let memberError = $state('');

	onMount(() => {
		if (isOwner) loadInvites();
	});

	async function loadInvites() {
		invitesLoading = true;
		invitesError = '';
		try {
			invites = await api.listInvites(Number(page.params.id));
		} catch (e) {
			invitesError = (e as Error).message;
		} finally {
			invitesLoading = false;
		}
	}

	async function generateInvite() {
		invitesError = '';
		try {
			await api.createInvite(Number(page.params.id));
			await loadInvites();
		} catch (e) {
			invitesError = (e as Error).message;
		}
	}

	async function revokeInvite(code: string) {
		invitesError = '';
		try {
			await api.revokeInvite(Number(page.params.id), code);
			await loadInvites();
		} catch (e) {
			invitesError = (e as Error).message;
		}
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
			await api.addTeamMember(Number(page.params.id), addUsername.trim());
			addUsername = '';
			await invalidateAll();
		} catch (e) {
			memberError = (e as Error).message;
		} finally {
			addingMember = false;
		}
	}

	async function setRole(memberId: number, role: TeamRole) {
		memberError = '';
		try {
			await api.setTeamMemberRole(Number(page.params.id), memberId, role);
			await invalidateAll();
		} catch (e) {
			memberError = (e as Error).message;
		}
	}

	async function removeMember(memberId: number) {
		memberError = '';
		try {
			await api.removeTeamMember(Number(page.params.id), memberId);
			await invalidateAll();
		} catch (e) {
			memberError = (e as Error).message;
		}
	}
</script>

<svelte:head>
	<title>Members - {team?.name ?? 'Team'} - shrl.io</title>
</svelte:head>

<h1 class="text-2xl font-semibold tracking-tight">Members</h1>

<div class="mt-6 space-y-6">
	<Card>
		<CardHeader class="flex-row items-center justify-between space-y-0">
			<CardTitle>Members</CardTitle>
			<span class="text-sm text-muted-foreground">{team?.members.length ?? 0}</span>
		</CardHeader>
		<CardContent>
			{#if memberError}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertDescription>{memberError}</AlertDescription>
				</Alert>
			{/if}
			{#if team?.members.length === 0}
				<p class="py-4 text-center text-sm text-muted-foreground">No members yet.</p>
			{:else}
				<ul class="divide-y">
					{#each team?.members ?? [] as member (member.id)}
						<li class="flex items-center justify-between gap-2 py-2.5">
							<span class="flex min-w-0 items-center gap-2">
								<span class="truncate font-medium">{member.username}</span>
								{#if member.username === myUsername}
									<span class="text-xs text-muted-foreground">(you)</span>
								{/if}
								{#if member.role === 'owner'}
									<Badge>Owner</Badge>
								{:else}
									<Badge variant="secondary">Member</Badge>
								{/if}
							</span>
							{#if canManage && member.username !== myUsername}
								<span class="flex shrink-0 gap-1.5">
									{#if member.role === 'owner'}
										<Button
											variant="outline"
											size="sm"
											onclick={() => setRole(member.id, 'member')}
										>
											Demote
										</Button>
									{:else}
										<Button
											variant="outline"
											size="sm"
											onclick={() => setRole(member.id, 'owner')}
										>
											Promote
										</Button>
									{/if}
									<Button
										variant="ghost"
										size="icon-sm"
										title="Remove member"
										onclick={() => removeMember(member.id)}
									>
										<Trash2 class="size-4" />
									</Button>
								</span>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
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
				<CardTitle>Invite Codes</CardTitle>
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
					<p class="py-2 text-center text-sm text-muted-foreground">No outstanding invite codes.</p>
				{:else}
					<ul class="divide-y">
						{#each invites as inv (inv.id)}
							<li class="flex items-center justify-between gap-2 py-2.5">
								<code class="rounded bg-muted px-2 py-1 text-sm font-semibold tracking-wide">
									{inv.code}
								</code>
								<span class="flex shrink-0 gap-1.5">
									<Button variant="outline" size="sm" onclick={() => copyCode(inv.code)}>
										<Copy class="size-4" />
										{copied === inv.code ? 'Copied' : 'Copy'}
									</Button>
									<Button
										variant="ghost"
										size="sm"
										title="Revoke code"
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
</div>
