export function isValidEmail(email: string): boolean {
	return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidPhone(phone: string): boolean {
	return /^\+?[1-9]\d{1,14}$/.test(phone.replace(/[\s\-()]/g, ''));
}

export function isRequired(value: string): boolean {
	return value.trim().length > 0;
}

export function maxLength(value: string, max: number): boolean {
	return value.length <= max;
}

export function minLength(value: string, min: number): boolean {
	return value.trim().length >= min;
}
