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
		<label for={inputId} class="block text-sm font-medium text-slate-700">
			{label}
			{#if required}<span class="text-red-500">*</span>{/if}
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
		class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-red-300 text-red-900 focus:border-red-500 focus:ring-red-500'
			: 'border-slate-300 text-slate-900 focus:border-indigo-500 focus:ring-indigo-500'}"
	/>
	{#if error}
		<p class="text-sm text-red-600">{error}</p>
	{:else if helper}
		<p class="text-sm text-slate-500">{helper}</p>
	{/if}
</div>
