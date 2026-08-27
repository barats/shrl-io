<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select';
	import {
		RANGE_PRESETS,
		presetLabel,
		rangeForPreset,
		type DateRange,
		type RangePreset
	} from '$lib/dashboard';

	let {
		value,
		onchange
	}: {
		value: DateRange;
		onchange: (range: DateRange) => void;
	} = $props();

	let preset = $state<RangePreset>('7d');
	let from = $state('');
	let to = $state('');

	$effect(() => {
		preset = value.preset;
		from = value.from;
		to = value.to;
	});

	function pickPreset(p: string | undefined) {
		if (!p) return;
		onchange(rangeForPreset(p as RangePreset));
	}

	function setFrom(f: string) {
		if (!f) return;
		onchange({ preset: 'custom', from: f, to });
	}

	function setTo(t: string) {
		if (!t) return;
		onchange({ preset: 'custom', from, to: t });
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	<Select type="single" bind:value={preset} onValueChange={pickPreset}>
		<SelectTrigger class="w-44">
			<span data-slot="select-value">{presetLabel(preset)}</span>
		</SelectTrigger>
		<SelectContent>
			{#each RANGE_PRESETS as p (p.value)}
				<SelectItem value={p.value} label={p.label} />
			{/each}
			<SelectItem value="custom" label="Custom range" />
		</SelectContent>
	</Select>
	{#if preset === 'custom'}
		<div class="flex items-center gap-2">
			<div class="flex flex-col gap-1">
				<Label for="range-from" class="text-xs text-muted-foreground">From</Label>
				<Input
					id="range-from"
					type="date"
					bind:value={from}
					onchange={(e) => setFrom((e.currentTarget as HTMLInputElement).value)}
					class="w-40"
				/>
			</div>
			<span class="text-sm text-muted-foreground">to</span>
			<div class="flex flex-col gap-1">
				<Label for="range-to" class="text-xs text-muted-foreground">To</Label>
				<Input
					id="range-to"
					type="date"
					bind:value={to}
					onchange={(e) => setTo((e.currentTarget as HTMLInputElement).value)}
					class="w-40"
				/>
			</div>
		</div>
	{/if}
</div>
