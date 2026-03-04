<script lang="ts">
	import { onMount } from 'svelte';
	import type { PublicEvent } from '$lib/types';

	interface Props {
		event: PublicEvent;
		shareToken: string;
	}

	let { event, shareToken }: Props = $props();
	let open = $state(false);
	let dropdownRef: HTMLDivElement = $state(undefined as unknown as HTMLDivElement);

	const googleUrl = $derived(buildGoogleCalendarUrl(event));
	const icsUrl = $derived(`/api/v1/rsvp/public/${shareToken}/calendar.ics`);

	function buildGoogleCalendarUrl(ev: PublicEvent): string {
		const start = toGoogleDate(ev.eventDate);
		const end = ev.endDate ? toGoogleDate(ev.endDate) : toGoogleDate(ev.eventDate, 2);
		const params = new URLSearchParams({
			action: 'TEMPLATE',
			text: ev.title,
			dates: `${start}/${end}`,
			location: ev.location,
			details: ev.description.slice(0, 1500)
		});
		return `https://calendar.google.com/calendar/render?${params.toString()}`;
	}

	function toGoogleDate(iso: string, addHours = 0): string {
		const d = new Date(iso);
		if (addHours) d.setHours(d.getHours() + addHours);
		return d.toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '');
	}

	function handleClickOutside(e: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(e.target as Node)) {
			open = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') open = false;
	}

	onMount(() => {
		document.addEventListener('click', handleClickOutside);
		return () => document.removeEventListener('click', handleClickOutside);
	});
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="relative" role="group" bind:this={dropdownRef} onkeydown={handleKeydown}>
	<button
		onclick={() => (open = !open)}
		aria-expanded={open}
		aria-haspopup="true"
		class="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-indigo-500/40 transition-colors"
	>
		<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
			/>
		</svg>
		Add to Calendar
	</button>
	{#if open}
		<div
			class="absolute left-0 z-10 mt-1 w-56 rounded-lg bg-white shadow-lg border border-slate-200 py-1"
			role="menu"
			aria-label="Calendar options"
		>
			<a
				href={googleUrl}
				target="_blank"
				rel="noopener noreferrer"
				role="menuitem"
				class="flex items-center gap-3 px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50 focus:bg-slate-50 focus:outline-none"
			>
				<svg class="h-4 w-4 text-slate-400" viewBox="0 0 24 24" fill="currentColor">
					<path
						d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"
					/>
				</svg>
				Google Calendar
			</a>
			<a
				href={icsUrl}
				download
				role="menuitem"
				class="flex items-center gap-3 px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50 focus:bg-slate-50 focus:outline-none"
			>
				<svg class="h-4 w-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2z"
					/>
				</svg>
				Apple Calendar (.ics)
			</a>
			<a
				href={icsUrl}
				download
				role="menuitem"
				class="flex items-center gap-3 px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50 focus:bg-slate-50 focus:outline-none"
			>
				<svg class="h-4 w-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
					/>
				</svg>
				Outlook (.ics)
			</a>
		</div>
	{/if}
</div>
