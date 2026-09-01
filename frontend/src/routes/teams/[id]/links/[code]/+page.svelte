<script lang="ts">
	import { page } from '$app/state';
	import LinkDetail from '$lib/components/LinkDetail.svelte';

	const teamId = $derived(page.params.id ?? '');
	const code = $derived(page.params.code as string);
</script>

<LinkDetail {code} {teamId} />
