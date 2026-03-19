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
		<label for={selectId} class="block text-sm font-medium text-neutral-700">
			{label}
			{#if required}<span class="text-error">*</span>{/if}
		</label>
	{/if}
	<select
		id={selectId}
		{name}
		bind:value
		{required}
		{disabled}
		class="block w-full rounded-md border px-3 py-2 text-sm shadow-sm transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-error-light text-error focus:border-error focus:ring-error'
			: 'border-neutral-300 text-neutral-900 focus:border-primary focus:ring-primary'} disabled:bg-neutral-50 disabled:text-neutral-500"
	>
		<option value="" disabled>{placeholder}</option>
		{#each options as opt}
			<option value={opt.value}>{opt.label}</option>
		{/each}
	</select>
	{#if error}
		<p class="text-sm text-error">{error}</p>
	{/if}
</div>
