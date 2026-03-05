import type { ApiError } from '$lib/types';

const BASE_URL = '/api/v1';
const TOKEN_KEY = 'openrsvp_session';
const CSRF_COOKIE = 'csrf_token';
const CSRF_HEADER = 'X-CSRF-Token';

/** Read a cookie value by name. Returns empty string if not found. */
function getCookie(name: string): string {
	if (typeof document === 'undefined') return '';
	const match = document.cookie.match(new RegExp('(?:^|;\\s*)' + name + '=([^;]*)'));
	return match ? decodeURIComponent(match[1]) : '';
}

/** Methods that mutate state and require CSRF protection. */
const MUTATION_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

class ApiClient {
	private token: string = '';

	constructor() {
		if (typeof window !== 'undefined') {
			this.token = localStorage.getItem(TOKEN_KEY) || '';
		}
	}

	setToken(token: string) {
		this.token = token;
		if (typeof window !== 'undefined') {
			if (token) {
				localStorage.setItem(TOKEN_KEY, token);
			} else {
				localStorage.removeItem(TOKEN_KEY);
			}
		}
	}

	getToken(): string {
		return this.token;
	}

	async request<T>(path: string, options: RequestInit = {}): Promise<T> {
		const url = `${BASE_URL}${path}`;
		const method = (options.method || 'GET').toUpperCase();
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			...((options.headers as Record<string, string>) || {})
		};

		if (this.token) {
			headers['Authorization'] = `Bearer ${this.token}`;
		}

		if (MUTATION_METHODS.has(method)) {
			const csrfToken = getCookie(CSRF_COOKIE);
			if (csrfToken) {
				headers[CSRF_HEADER] = csrfToken;
			}
		}

		const response = await fetch(url, {
			...options,
			headers
		});

		if (!response.ok) {
			if (response.status === 429) {
				const retryAfter = response.headers.get('Retry-After');
				const error: ApiError = {
					error: 'rate_limited',
					message: retryAfter
						? `Too many requests. Please wait ${retryAfter} seconds and try again.`
						: 'Too many requests. Please wait a moment and try again.',
					status: 429
				};
				throw error;
			}
			const error: ApiError = await response.json().catch(() => ({
				error: 'unknown',
				message: response.statusText,
				status: response.status
			}));
			throw error;
		}

		if (response.status === 204) {
			return undefined as T;
		}

		return response.json();
	}

	get<T>(path: string) {
		return this.request<T>(path, { method: 'GET' });
	}

	post<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'POST',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	put<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'PUT',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	patch<T>(path: string, body?: unknown) {
		return this.request<T>(path, {
			method: 'PATCH',
			body: body ? JSON.stringify(body) : undefined
		});
	}

	delete<T>(path: string) {
		return this.request<T>(path, { method: 'DELETE' });
	}

	async upload<T>(path: string, file: File): Promise<T> {
		const url = `${BASE_URL}${path}`;
		const formData = new FormData();
		formData.append('image', file);

		const headers: Record<string, string> = {};
		if (this.token) {
			headers['Authorization'] = `Bearer ${this.token}`;
		}
		// Do NOT set Content-Type — browser sets multipart boundary automatically.

		// Upload is always a POST (mutation) — include CSRF token.
		const csrfToken = getCookie(CSRF_COOKIE);
		if (csrfToken) {
			headers[CSRF_HEADER] = csrfToken;
		}

		const response = await fetch(url, {
			method: 'POST',
			headers,
			body: formData
		});

		if (!response.ok) {
			if (response.status === 429) {
				const retryAfter = response.headers.get('Retry-After');
				const error: ApiError = {
					error: 'rate_limited',
					message: retryAfter
						? `Too many requests. Please wait ${retryAfter} seconds and try again.`
						: 'Too many requests. Please wait a moment and try again.',
					status: 429
				};
				throw error;
			}
			const error: ApiError = await response.json().catch(() => ({
				error: 'unknown',
				message: response.statusText,
				status: response.status
			}));
			throw error;
		}

		return response.json();
	}
}

export const api = new ApiClient();
