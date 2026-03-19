<script lang="ts">
	interface Props {
		label?: string;
		name?: string;
		value?: string;
		min?: string;
		max?: string;
		error?: string;
		helper?: string;
		required?: boolean;
		class?: string;
	}

	let {
		label,
		name = 'datetime',
		value = $bindable(''),
		min,
		max,
		error = '',
		helper = '',
		required = false,
		class: className = ''
	}: Props = $props();

	let inputId = $derived(`datetime-${name}`);
</script>

<div class="space-y-1 {className}">
	{#if label}
		<label for={inputId} class="block text-sm font-medium text-neutral-700">
			{label}
			{#if required}<span class="text-error">*</span>{/if}
		</label>
	{/if}
	<input
		id={inputId}
		{name}
		type="datetime-local"
		bind:value
		{min}
		{max}
		{required}
		class="block w-full rounded-md border px-3 py-2 text-sm shadow-sm transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-error-light text-error focus:border-error focus:ring-error'
			: 'border-neutral-300 text-neutral-900 focus:border-primary focus:ring-primary'}"
	/>
	{#if error}
		<p class="text-sm text-error">{error}</p>
	{:else if helper}
		<p class="text-sm text-neutral-500">{helper}</p>
	{/if}
</div>
