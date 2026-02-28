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
		<label for={textareaId} class="block text-sm font-medium text-slate-700">
			{label}
			{#if required}<span class="text-red-500">*</span>{/if}
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
		class="block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-offset-0 {error
			? 'border-red-300 text-red-900 placeholder-red-300 focus:border-red-500 focus:ring-red-500'
			: 'border-slate-300 text-slate-900 placeholder-slate-400 focus:border-indigo-500 focus:ring-indigo-500'} disabled:bg-slate-50 disabled:text-slate-500"
	></textarea>
	{#if error}
		<p class="text-sm text-red-600">{error}</p>
	{:else if helper}
		<p class="text-sm text-slate-500">{helper}</p>
	{/if}
</div>
