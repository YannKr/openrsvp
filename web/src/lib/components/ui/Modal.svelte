<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		open?: boolean;
		title?: string;
		children: Snippet;
		actions?: Snippet;
	}

	let { open = $bindable(false), title = '', children, actions }: Props = $props();

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			open = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			open = false;
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		onkeydown={handleKeydown}
	>
		<!-- Backdrop -->
		<button
			type="button"
			class="absolute inset-0 bg-black/50 transition-opacity"
			onclick={() => (open = false)}
			aria-label="Close modal"
		></button>

		<!-- Panel -->
		<div class="relative bg-white rounded-xl shadow-xl max-w-lg w-full max-h-[90vh] overflow-y-auto">
			{#if title}
				<div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
					<h2 class="text-lg font-semibold text-slate-900">{title}</h2>
					<button
						type="button"
						onclick={() => (open = false)}
						class="text-slate-400 hover:text-slate-600 transition-colors"
						aria-label="Close"
					>
						<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M6 18L18 6M6 6l12 12"
							/>
						</svg>
					</button>
				</div>
			{/if}

			<div class="px-6 py-4">
				{@render children()}
			</div>

			{#if actions}
				<div class="border-t border-slate-200 px-6 py-4 flex justify-end gap-3">
					{@render actions()}
				</div>
			{/if}
		</div>
	</div>
{/if}
