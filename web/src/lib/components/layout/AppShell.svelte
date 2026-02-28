<script lang="ts">
	import type { Snippet } from 'svelte';
	import Navbar from './Navbar.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="min-h-screen bg-slate-50">
	<Navbar />
	<main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{@render children()}
	</main>
</div>
