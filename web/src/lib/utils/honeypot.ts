export function isHoneypotFilled(formData: FormData): boolean {
	const honeypotValue = formData.get('website');
	return honeypotValue !== null && honeypotValue !== '';
}

export function getHoneypotFieldName(): string {
	return 'website';
}
