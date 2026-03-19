<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
		size?: 'sm' | 'md' | 'lg';
		disabled?: boolean;
		loading?: boolean;
		type?: 'button' | 'submit' | 'reset';
		href?: string;
		class?: string;
		onclick?: (e: MouseEvent) => void;
		children: Snippet;
	}

	let {
		variant = 'primary',
		size = 'md',
		disabled = false,
		loading = false,
		type = 'button',
		href,
		class: className = '',
		onclick,
		children
	}: Props = $props();

	const baseClasses =
		'inline-flex items-center justify-center font-medium rounded-md transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

	const variantClasses: Record<string, string> = {
		primary: 'bg-primary text-white hover:bg-primary-hover focus:ring-primary',
		secondary: 'bg-primary-light text-primary hover:bg-primary-lighter focus:ring-primary',
		outline: 'border border-neutral-300 text-neutral-700 hover:bg-neutral-50 focus:ring-primary',
		ghost: 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 focus:ring-primary',
		danger: 'bg-error text-white hover:bg-red-700 focus:ring-error'
	};

	const sizeClasses: Record<string, string> = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2 text-sm',
		lg: 'px-6 py-3 text-base'
	};
</script>

{#if href}
	<a href={href} class="{baseClasses} {variantClasses[variant]} {sizeClasses[size]} {className}">
		{#if loading}
			<svg
				class="animate-spin -ml-1 mr-2 h-4 w-4"
				xmlns="http://www.w3.org/2000/svg"
				fill="none"
				viewBox="0 0 24 24"
			>
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
				></circle>
				<path
					class="opacity-75"
					fill="currentColor"
					d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
			</svg>
		{/if}
		{@render children()}
	</a>
{:else}
	<button
		{type}
		{onclick}
		disabled={disabled || loading}
		class="{baseClasses} {variantClasses[variant]} {sizeClasses[size]} {className}"
	>
		{#if loading}
			<svg
				class="animate-spin -ml-1 mr-2 h-4 w-4"
				xmlns="http://www.w3.org/2000/svg"
				fill="none"
				viewBox="0 0 24 24"
			>
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
				></circle>
				<path
					class="opacity-75"
					fill="currentColor"
					d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
			</svg>
		{/if}
		{@render children()}
	</button>
{/if}
