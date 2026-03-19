<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'success' | 'warning' | 'error' | 'info' | 'neutral';
		children: Snippet;
	}

	let { variant = 'neutral', children }: Props = $props();

	const variantClasses: Record<string, string> = {
		success: 'bg-success-light text-success',
		warning: 'bg-warning-light text-warning',
		error: 'bg-error-light text-error',
		info: 'bg-info-light text-info',
		neutral: 'bg-neutral-100 text-neutral-700'
	};

	const dotClasses: Record<string, string> = {
		success: 'bg-success',
		warning: 'bg-warning',
		error: 'bg-error',
		info: 'bg-info',
		neutral: 'bg-neutral-400'
	};
</script>

<span
	class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium {variantClasses[variant]}"
>
	<span class="h-1.5 w-1.5 rounded-full {dotClasses[variant]}"></span>
	{@render children()}
</span>
