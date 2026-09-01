import { browser } from '$app/environment';

export type Theme = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'shrl:theme';

function systemDark(): boolean {
	if (!browser) return false;
	return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function readInitial(): Theme {
	if (!browser) return 'system';
	const saved = localStorage.getItem(STORAGE_KEY);
	if (saved === 'light' || saved === 'dark' || saved === 'system') return saved;
	return 'system';
}

// Exported as one reactive object so callers can read and reassign its
// properties; a single const-wrapped $state object keeps TypeScript happy
// where exporting a primitive $state does not.
export const themeState = $state<{ theme: Theme; effectiveDark: boolean }>({
	theme: readInitial(),
	effectiveDark: false
});

function apply() {
	if (!browser) return;
	const dark =
		themeState.theme === 'dark' || (themeState.theme === 'system' && systemDark());
	themeState.effectiveDark = dark;
	document.documentElement.classList.toggle('dark', dark);
}

// Apply on first import so the header toggle, charts, and the page agree with
// the pre-paint script in app.html (which prevents a light-mode flash).
apply();

// Keep following the OS while in system mode (no explicit choice saved).
if (browser) {
	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', apply);
}

export function setTheme(t: Theme) {
	themeState.theme = t;
	if (browser) localStorage.setItem(STORAGE_KEY, t);
	apply();
}

export function toggleTheme() {
	setTheme(themeState.effectiveDark ? 'light' : 'dark');
}
