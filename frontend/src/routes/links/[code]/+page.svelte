<script lang="ts">
	import { page } from '$app/state';
	import LinkDetail from '$lib/components/LinkDetail.svelte';

	const code = $derived(page.params.code as string);
</script>

<LinkDetail {code} />
