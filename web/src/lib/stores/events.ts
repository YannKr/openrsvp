import { writable } from 'svelte/store';
import type { Event } from '$lib/types';

export const events = writable<Event[]>([]);
export const currentEvent = writable<Event | null>(null);
export const eventsLoading = writable(false);
