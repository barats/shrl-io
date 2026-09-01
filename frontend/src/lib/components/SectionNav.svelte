<script lang="ts">
	import { tick } from 'svelte';

	let {
		sections,
		label
	}: {
		sections: { id: string; label: string }[];
		label: string;
	} = $props();

	// Empty until the observer's initial callback resolves the visible
	// section; avoids capturing the initial value of the sections prop.
	let activeSection = $state('');

	// Set while a rail click drives the scroll; the observer must not fight it.
	let autoScrolling = $state(false);
	let scrollTimer: ReturnType<typeof setTimeout> | undefined;

	function scrollToSection(id: string) {
		activeSection = id;
		history.replaceState(null, '', `#${id}`);
		const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		document.getElementById(id)?.scrollIntoView({ behavior: reduced ? 'auto' : 'smooth' });
		autoScrolling = true;
		clearTimeout(scrollTimer);
		scrollTimer = setTimeout(() => (autoScrolling = false), 800);
	}

	// Watch the anchored sections and keep the active rail item in sync with
	// the scroll position (IntersectionObserver-driven, no scroll listeners).
	$effect(() => {
		// Read sections synchronously so a late-appearing section (e.g. API
		// keys once /me resolves) re-runs this effect and gets observed.
		const ids = sections.map((s) => s.id);
		const observer = new IntersectionObserver(
			() => {
				if (autoScrolling) return;
				const doc = document.documentElement;
				// Active = last section whose top passed the upper viewport band;
				// on a scrollable page at the very bottom it is the last section.
				const scrollable = doc.scrollHeight > window.innerHeight + 40;
				if (scrollable && window.scrollY > 0 && window.innerHeight + window.scrollY >= doc.scrollHeight - 2) {
					activeSection = ids[ids.length - 1];
					return;
				}
				let current = ids[0];
				for (const id of ids) {
					const el = document.getElementById(id);
					if (el && el.getBoundingClientRect().top <= 140) current = id;
				}
				activeSection = current;
			},
			{ rootMargin: '0px 0px -75% 0px' }
		);
		// The sections are rendered after this nav in the page; wait for the
		// initial render to settle before attaching.
		void tick().then(() => {
			for (const id of ids) {
				const el = document.getElementById(id);
				if (el) observer.observe(el);
			}
		});
		return () => {
			observer.disconnect();
			clearTimeout(scrollTimer);
		};
	});
</script>

<nav class="sticky top-8 hidden self-start md:block" aria-label={label}>
	<ul class="space-y-1">
		{#each sections as s (s.id)}
			<li>
				<a
					href={'#' + s.id}
					aria-current={activeSection === s.id ? 'true' : undefined}
					onclick={(e) => {
						e.preventDefault();
						scrollToSection(s.id);
					}}
					class="block border-l-2 py-1.5 pl-3 text-sm transition-colors {activeSection ===
					s.id
						? 'border-foreground font-medium text-foreground'
						: 'border-transparent text-muted-foreground hover:border-border hover:text-foreground'}"
				>
					{s.label}
				</a>
			</li>
		{/each}
	</ul>
</nav>
