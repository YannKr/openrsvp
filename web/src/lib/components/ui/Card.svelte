<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		class?: string;
		children: Snippet;
		header?: Snippet;
	}

	let { class: className = '', children, header }: Props = $props();
</script>

<div class="bg-surface rounded-lg shadow-sm border border-neutral-200 transition-all duration-short ease-out hover:-translate-y-0.5 hover:shadow-md {className}">
	{#if header}
		<div class="border-b border-neutral-200 px-6 py-4">
			{@render header()}
		</div>
	{/if}
	<div class="px-6 py-4">
		{@render children()}
	</div>
</div>
