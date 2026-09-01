<script lang="ts">
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Download, TriangleAlert } from '@lucide/svelte';

	interface Props {
		hostname: string;
		code: string;
	}

	let { hostname, code }: Props = $props();

	// The QR always points at the redirector's public URL, never the admin UI.
	const shortUrl = $derived(`https://${hostname}/${code}`);
	let dataUrl = $state('');
	let error = $state('');

	onMount(render);

	async function render() {
		error = '';
		try {
			// Generated entirely in the browser: nothing leaves the page.
			dataUrl = await QRCode.toDataURL(shortUrl, {
				errorCorrectionLevel: 'M',
				margin: 2,
				width: 1024
			});
		} catch (e) {
			error = (e as Error).message;
		}
	}

	function downloadPng() {
		if (!dataUrl) return;
		const a = document.createElement('a');
		a.href = dataUrl;
		a.download = `${hostname}-${code}.png`;
		document.body.appendChild(a);
		a.click();
		a.remove();
	}
</script>

<Card>
	<CardHeader>
		<CardTitle>QR code</CardTitle>
		<CardDescription>Scan to open this Link.</CardDescription>
	</CardHeader>
	<CardContent class="flex flex-wrap items-center gap-4">
		{#if dataUrl}
			<img
				src={dataUrl}
				alt={`QR code for ${shortUrl}`}
				class="size-32 shrink-0 rounded-lg border bg-white p-1"
			/>
		{:else if error}
			<Alert variant="destructive" class="min-w-0 flex-1">
				<TriangleAlert class="size-4" />
				<AlertTitle>Could not generate QR code</AlertTitle>
				<AlertDescription>{error}</AlertDescription>
			</Alert>
		{:else}
			<div class="size-32 shrink-0 animate-pulse rounded-lg border bg-muted"></div>
		{/if}
		<div class="min-w-0 flex-1 space-y-2">
			<p class="truncate text-sm font-medium">{shortUrl}</p>
			<Button class="gap-2" disabled={!dataUrl} onclick={downloadPng}>
				<Download class="size-4" /> Download PNG
			</Button>
		</div>
	</CardContent>
</Card>
