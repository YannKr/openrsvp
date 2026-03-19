<script lang="ts">
	interface Props {
		label?: string;
		name: string;
		type?: 'text' | 'email' | 'tel' | 'password' | 'url' | 'number';
		value?: string;
		placeholder?: string;
		error?: string;
		helper?: string;
		required?: boolean;
		disabled?: boolean;
		class?: string;
		oninput?: (e: Event) => void;
	}

	let {
		label,
		name,
		type = 'text',
		value = $bindable(''),
		placeholder = '',
		error = '',
		helper = '',
		required = false,
		disabled = false,
		class: className = '',
		oninput
	}: Props = $props();

	let inputId = $derived(`input-${name}`);
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
		{type}
		bind:value
		{placeholder}
		{required}
		{disabled}
		{oninput}
		class="block w-full rounded-md border px-3 py-2 text-sm shadow-sm transition-colors duration-short ease-out focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-error-light text-error placeholder-error-light focus:border-error focus:ring-error'
			: 'border-neutral-300 text-neutral-900 placeholder-neutral-400 focus:border-primary focus:ring-primary'} disabled:bg-neutral-50 disabled:text-neutral-500"
	/>
	{#if error}
		<p class="text-sm text-error">{error}</p>
	{:else if helper}
		<p class="text-sm text-neutral-500">{helper}</p>
	{/if}
</div>
