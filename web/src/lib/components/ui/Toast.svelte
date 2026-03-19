<script lang="ts">
	import { toast } from '$lib/stores/toast';

	const typeClasses: Record<string, string> = {
		success: 'bg-success',
		error: 'bg-error',
		info: 'bg-info',
		warning: 'bg-warning'
	};

	const typeIcons: Record<string, string> = {
		success: 'M5 13l4 4L19 7',
		error: 'M6 18L18 6M6 6l12 12',
		info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
		warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z'
	};
</script>

{#if $toast.length > 0}
	<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
		{#each $toast as t (t.id)}
			<div
				class="px-4 py-3 rounded-md shadow-lg text-white text-sm max-w-sm {typeClasses[t.type]}"
			>
				<div class="flex items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d={typeIcons[t.type]}
							/>
						</svg>
						<span>{t.message}</span>
					</div>
					<button
						type="button"
						onclick={() => toast.remove(t.id)}
						class="text-white/80 hover:text-white shrink-0"
						aria-label="Dismiss"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M6 18L18 6M6 6l12 12"
							/>
						</svg>
					</button>
				</div>
			</div>
		{/each}
	</div>
{/if}
