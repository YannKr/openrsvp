<script lang="ts">
	interface SelectOption {
		value: string;
		label: string;
	}

	interface Props {
		label?: string;
		name: string;
		value?: string;
		options: SelectOption[];
		error?: string;
		required?: boolean;
		disabled?: boolean;
		placeholder?: string;
		class?: string;
	}

	let {
		label,
		name,
		value = $bindable(''),
		options,
		error = '',
		required = false,
		disabled = false,
		placeholder = 'Select an option...',
		class: className = ''
	}: Props = $props();

	let selectId = $derived(`select-${name}`);
</script>

<div class="space-y-1 {className}">
	{#if label}
		<label for={selectId} class="block text-sm font-medium text-slate-700">
			{label}
			{#if required}<span class="text-red-500">*</span>{/if}
		</label>
	{/if}
	<select
		id={selectId}
		{name}
		bind:value
		{required}
		{disabled}
		class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-red-300 text-red-900 focus:border-red-500 focus:ring-red-500'
			: 'border-slate-300 text-slate-900 focus:border-indigo-500 focus:ring-indigo-500'} disabled:bg-slate-50 disabled:text-slate-500"
	>
		<option value="" disabled>{placeholder}</option>
		{#each options as opt}
			<option value={opt.value}>{opt.label}</option>
		{/each}
	</select>
	{#if error}
		<p class="text-sm text-red-600">{error}</p>
	{/if}
</div>
