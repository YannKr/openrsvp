import { writable, derived } from 'svelte/store';
import type { Organizer } from '$lib/types';

export const currentUser = writable<Organizer | null>(null);
export const isAuthenticated = derived(currentUser, ($user) => $user !== null);
export const isLoading = writable(true);
