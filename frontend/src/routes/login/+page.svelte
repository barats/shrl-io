<script lang="ts">
	import { api } from '$lib/api';
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
	import { TriangleAlert } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		error = '';
		try {
			await api.login(username, password);
			// Full navigation so the root layout re-runs its server load with the
			// now-valid session; a client-side goto would keep the unauthenticated
			// layout data (no username / admin nav) until the next reload.
			window.location.assign('/');
		} catch (e) {
			error = (e as Error).message;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in — shrl.io</title>
</svelte:head>

<div class="flex min-h-[70vh] items-center justify-center">
	<Card class="w-full max-w-sm">
		<CardHeader>
			<CardTitle class="text-xl">shrl.io</CardTitle>
			<CardDescription>Sign in to manage your Links.</CardDescription>
		</CardHeader>
		<CardContent>
			{#if error}
				<Alert variant="destructive" class="mb-4">
					<TriangleAlert class="size-4" />
					<AlertTitle>Sign in failed</AlertTitle>
					<AlertDescription>{error}</AlertDescription>
				</Alert>
			{/if}
			<form onsubmit={submit} class="space-y-4">
				<div class="space-y-2">
					<Label for="username">Username</Label>
					<Input
						id="username"
						type="text"
						bind:value={username}
						autocomplete="username"
						placeholder="admin"
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						autocomplete="current-password"
						placeholder="••••••••••"
						required
					/>
				</div>
				<Button type="submit" class="w-full" disabled={submitting}>
					{submitting ? 'Signing in…' : 'Sign in'}
				</Button>
			</form>
		</CardContent>
	</Card>
</div>
