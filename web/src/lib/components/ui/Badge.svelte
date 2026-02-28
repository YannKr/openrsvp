<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'success' | 'warning' | 'error' | 'info' | 'neutral';
		children: Snippet;
	}

	let { variant = 'neutral', children }: Props = $props();

	const variantClasses: Record<string, string> = {
		success: 'bg-green-50 text-green-700',
		warning: 'bg-yellow-50 text-yellow-700',
		error: 'bg-red-50 text-red-700',
		info: 'bg-blue-50 text-blue-700',
		neutral: 'bg-slate-100 text-slate-700'
	};

	const dotClasses: Record<string, string> = {
		success: 'bg-green-500',
		warning: 'bg-yellow-500',
		error: 'bg-red-500',
		info: 'bg-blue-500',
		neutral: 'bg-slate-400'
	};
</script>

<span
	class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium {variantClasses[variant]}"
>
	<span class="h-1.5 w-1.5 rounded-full {dotClasses[variant]}"></span>
	{@render children()}
</span>
