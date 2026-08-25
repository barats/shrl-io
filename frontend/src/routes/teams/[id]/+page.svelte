<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import type { InviteCode, Link, TeamDetail, TeamRole, User } from '$lib/types';
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
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select';
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
		KeyRound,
		Link2,
		LogOut,
		Plus,
		Trash2,
		TriangleAlert,
		UserPlus
	} from '@lucide/svelte';

	const teamId = $derived(Number(page.params.id));
	const isAdmin = $derived(page.data.user?.isAdmin ?? false);

	let team = $state<TeamDetail | null>(null);
	let me = $state<User | null>(null);
	let loading = $state(true);
	let error = $state('');

	let hostnames = $state<string[]>([]);
	let selectedHostname = $state('');
	let teamLinks = $state<Link[]>([]);
	let linksLoading = $state(true);
	let linksError = $state('');

	let invites = $state<InviteCode[]>([]);
	let invitesLoading = $state(true);
	let invitesError = $state('');
	let copied = $state('');

	let addUsername = $state('');
	let addingMember = $state(false);
	let memberError = $state('');

	let createDestination = $state('');
	let createRemark = $state('');
	let createHostname = $state('');
	let creating = $state(false);
	let createError = $state('');

	const myRole = $derived<TeamRole | undefined>(
		team?.members.find((m) => m.id === me?.id)?.role
	);
	const isOwner = $derived(myRole === 'owner');
	const canManage = $derived(isOwner || isAdmin);

	onMount(async () => {
		try {
			const cfg = await api.config();
			const [t, u, hs] = await Promise.all([api.getTeam(teamId), api.me(), api.hostnames()]);
			team = t;
			me = u;
			hostnames = [...new Set([cfg.defaultHostname, ...hs])].sort();
			selectedHostname = cfg.defaultHostname;
			createHostname = cfg.defaultHostname;
			await Promise.all([loadLinks(), loadInvites()]);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	});

	async function loadLinks() {
		linksLoading = true;
		linksError = '';
		try {
			teamLinks = await api.listTeamLinks(teamId, selectedHostname);
		} catch (e) {
			linksError = (e as Error).message;
		} finally {
			linksLoading = false;
		}
	}

	async function onHostnameChange(value: string | undefined) {
		if (value === undefined) return;
		selectedHostname = value;
		createHostname = value;
		await loadLinks();
	}

	async function loadInvites() {
		if (!isOwner) {
			invitesLoading = false;
			return;
		}
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

	async function revokeInvite(code: string) {
		invitesError = '';
		try {
			await api.revokeInvite(teamId, code);
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
			await api.addTeamMember(teamId, addUsername.trim());
			addUsername = '';
			team = await api.getTeam(teamId);
		} catch (e) {
			memberError = (e as Error).message;
		} finally {
			addingMember = false;
		}
	}

	async function setRole(memberId: number, role: TeamRole) {
		memberError = '';
		try {
			await api.setTeamMemberRole(teamId, memberId, role);
			team = await api.getTeam(teamId);
		} catch (e) {
			memberError = (e as Error).message;
		}
	}

	async function removeMember(memberId: number) {
		memberError = '';
		try {
			await api.removeTeamMember(teamId, memberId);
			team = await api.getTeam(teamId);
		} catch (e) {
			memberError = (e as Error).message;
		}
	}

	async function leave() {
		if (!me) return;
		if (!window.confirm('Leave this Team? Your Links stay with the Team.')) return;
		try {
			await api.removeTeamMember(teamId, me.id);
			await goto('/teams');
		} catch (e) {
			error = (e as Error).message;
		}
	}

	async function removeTeam() {
		if (!window.confirm('Delete this Team? Its Links revert to Personal for their Creators.')) return;
		try {
			await api.deleteTeam(teamId);
			await goto('/teams');
		} catch (e) {
			error = (e as Error).message;
		}
	}

	async function createTeamLink() {
		creating = true;
		createError = '';
		try {
			await api.createTeamLink(teamId, {
				hostname: createHostname,
				destination: createDestination,
				remark: createRemark || undefined
			});
			createDestination = '';
			createRemark = '';
			await loadLinks();
		} catch (e) {
			createError = (e as Error).message;
		} finally {
			creating = false;
		}
	}
</script>

{#if loading}
	<div class="space-y-4">
		<Skeleton class="h-8 w-64" />
		<Skeleton class="h-40 w-full" />
	</div>
{:else if error}
	<Alert variant="destructive">
		<TriangleAlert class="size-4" />
		<AlertTitle>Failed to load Team</AlertTitle>
		<AlertDescription>{error}</AlertDescription>
	</Alert>
{:else if team}
	<div class="flex flex-wrap items-center gap-3">
		<h1 class="text-2xl font-semibold tracking-tight">{team.name}</h1>
		{#if myRole === 'owner'}
			<Badge>Owner</Badge>
		{:else if myRole === 'member'}
			<Badge variant="secondary">Member</Badge>
		{:else}
			<Badge variant="outline">Not a member</Badge>
		{/if}
		<div class="ml-auto flex gap-2">
			{#if myRole}
				<Button variant="outline" onclick={leave}>
					<LogOut class="size-4" /> Leave
				</Button>
			{/if}
			{#if isAdmin}
				<Button variant="destructive" onclick={removeTeam}>
					<Trash2 class="size-4" /> Delete Team
				</Button>
			{/if}
		</div>
	</div>
	<p class="mt-1 text-sm text-muted-foreground">
		Created {team.created_at.slice(0, 10)} · {team.members.length}{' '}
		{team.members.length === 1 ? 'member' : 'members'}
	</p>

	<div class="mt-6 grid gap-6 lg:grid-cols-2">
		<div class="space-y-6">
			<Card>
				<CardHeader class="flex-row items-center justify-between space-y-0">
					<CardTitle>Members</CardTitle>
					<span class="text-sm text-muted-foreground">{team.members.length}</span>
				</CardHeader>
				<CardContent>
					{#if memberError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertDescription>{memberError}</AlertDescription>
						</Alert>
					{/if}
					{#if team.members.length === 0}
						<p class="py-4 text-center text-sm text-muted-foreground">No members yet.</p>
					{:else}
						<ul class="divide-y">
							{#each team.members as member (member.id)}
								<li class="flex items-center justify-between gap-2 py-2.5">
									<span class="flex min-w-0 items-center gap-2">
										<span class="truncate font-medium">{member.username}</span>
										{#if member.id === me?.id}
											<span class="text-xs text-muted-foreground">(you)</span>
										{/if}
										{#if member.role === 'owner'}
											<Badge>Owner</Badge>
										{:else}
											<Badge variant="secondary">Member</Badge>
										{/if}
									</span>
									{#if canManage && member.id !== me?.id}
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

		<div class="space-y-6">
			<Card>
				<CardHeader class="flex-row items-center justify-between space-y-0">
					<CardTitle>Team Links</CardTitle>
					<div class="w-56">
						<Select type="single" bind:value={selectedHostname} onValueChange={onHostnameChange}>
							<SelectTrigger class="w-full">
								<span data-slot="select-value">{selectedHostname || 'Hostname'}</span>
							</SelectTrigger>
							<SelectContent>
								{#each hostnames as host (host)}
									<SelectItem value={host} label={host} />
								{/each}
							</SelectContent>
						</Select>
					</div>
				</CardHeader>
				<CardContent>
					{#if linksError}
						<Alert variant="destructive">
							<TriangleAlert class="size-4" />
							<AlertDescription>{linksError}</AlertDescription>
						</Alert>
					{:else if linksLoading}
						<div class="space-y-3">
							{#each [0, 1, 2] as i (i)}
								<Skeleton class="h-10 w-full" />
							{/each}
						</div>
					{:else if teamLinks.length === 0}
						<p class="py-8 text-center text-sm text-muted-foreground">
							No Links on <span class="font-medium">{selectedHostname}</span> yet. Create one below.
						</p>
					{:else}
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Code</TableHead>
									<TableHead>Destination</TableHead>
									<TableHead class="w-24">Status</TableHead>
									<TableHead class="w-36">Created</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{#each teamLinks as link (link.hostname + link.code)}
									<TableRow>
										<TableCell class="font-medium">
											<a
												href={`/teams/${teamId}/links/${encodeURIComponent(link.code)}?hostname=${encodeURIComponent(link.hostname)}`}
												class="inline-flex items-center gap-1.5 text-primary hover:underline"
											>
												<Link2 class="size-3.5" />
												{link.code}
											</a>
										</TableCell>
										<TableCell class="max-w-72 truncate text-muted-foreground">
											{link.destination}
										</TableCell>
										<TableCell>
											{#if link.disabled}
												<Badge variant="secondary">Disabled</Badge>
											{:else}
												<Badge>Active</Badge>
											{/if}
										</TableCell>
										<TableCell class="text-muted-foreground">
											{link.created_at.slice(0, 10)}
										</TableCell>
									</TableRow>
								{/each}
							</TableBody>
						</Table>
					{/if}
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Create a Link in this Team</CardTitle>
					<CardDescription>
						Shorten a Destination under a registered Hostname. The Team owns the Link.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{#if createError}
						<Alert variant="destructive" class="mb-4">
							<TriangleAlert class="size-4" />
							<AlertTitle>Could not create Link</AlertTitle>
							<AlertDescription>{createError}</AlertDescription>
						</Alert>
					{/if}
					<form
						onsubmit={(e) => {
							e.preventDefault();
							createTeamLink();
						}}
						class="space-y-4"
					>
						<div class="space-y-2">
							<Label for="team-link-hostname">Hostname</Label>
							<Select type="single" bind:value={createHostname}>
								<SelectTrigger id="team-link-hostname" class="w-full">
									<span data-slot="select-value">{createHostname || 'Hostname'}</span>
								</SelectTrigger>
								<SelectContent>
									{#each hostnames as host (host)}
										<SelectItem value={host} label={host} />
									{/each}
								</SelectContent>
							</Select>
						</div>
						<div class="space-y-2">
							<Label for="team-link-destination">Destination</Label>
							<Input
								id="team-link-destination"
								bind:value={createDestination}
								placeholder="https://example.com"
								required
							/>
						</div>
						<div class="space-y-2">
							<Label for="team-link-remark">Remark (optional)</Label>
							<Input id="team-link-remark" bind:value={createRemark} placeholder="What this Link is for" />
						</div>
						<Button type="submit" class="w-full" disabled={creating}>
							{#if creating}
								Creating…
							{:else}
								<Plus class="size-4" /> Create Link
							{/if}
						</Button>
					</form>
				</CardContent>
			</Card>
		</div>
	</div>
{/if}
