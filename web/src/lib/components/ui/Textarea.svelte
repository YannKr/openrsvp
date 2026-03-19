<script lang="ts">
	interface Props {
		label?: string;
		name: string;
		value?: string;
		placeholder?: string;
		error?: string;
		helper?: string;
		required?: boolean;
		disabled?: boolean;
		rows?: number;
		class?: string;
	}

	let {
		label,
		name,
		value = $bindable(''),
		placeholder = '',
		error = '',
		helper = '',
		required = false,
		disabled = false,
		rows = 4,
		class: className = ''
	}: Props = $props();

	let textareaId = $derived(`textarea-${name}`);
</script>

<div class="space-y-1 {className}">
	{#if label}
		<label for={textareaId} class="block text-sm font-medium text-neutral-700">
			{label}
			{#if required}<span class="text-error">*</span>{/if}
		</label>
	{/if}
	<textarea
		id={textareaId}
		{name}
		bind:value
		{placeholder}
		{required}
		{disabled}
		{rows}
		class="block w-full rounded-md border px-3 py-2 text-sm shadow-sm transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-error-light text-error placeholder-error-light focus:border-error focus:ring-error'
			: 'border-neutral-300 text-neutral-900 placeholder-neutral-400 focus:border-primary focus:ring-primary'} disabled:bg-neutral-50 disabled:text-neutral-500"
	></textarea>
	{#if error}
		<p class="text-sm text-error">{error}</p>
	{:else if helper}
		<p class="text-sm text-neutral-500">{helper}</p>
	{/if}
</div>
