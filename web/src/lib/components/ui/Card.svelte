<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		class?: string;
		children: Snippet;
		header?: Snippet;
	}

	let { class: className = '', children, header }: Props = $props();
</script>

<div class="bg-white rounded-xl shadow-sm border border-slate-200 {className}">
	{#if header}
		<div class="border-b border-slate-200 px-6 py-4">
			{@render header()}
		</div>
	{/if}
	<div class="px-6 py-4">
		{@render children()}
	</div>
</div>
