export interface Organizer {
	id: string;
	email: string;
	name: string;
	timezone: string;
	createdAt: string;
	updatedAt: string;
}

export interface Event {
	id: string;
	organizerId: string;
	title: string;
	description: string;
	eventDate: string;
	endDate?: string;
	location: string;
	timezone: string;
	retentionDays: number;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	status: 'draft' | 'published' | 'cancelled' | 'archived';
	shareToken: string;
	createdAt: string;
	updatedAt: string;
}

export interface InviteCard {
	id: string;
	eventId: string;
	templateId: string;
	heading: string;
	body: string;
	footer: string;
	primaryColor: string;
	secondaryColor: string;
	font: string;
	customData: Record<string, unknown>;
	createdAt: string;
	updatedAt: string;
}

export interface Attendee {
	id: string;
	eventId: string;
	name: string;
	email?: string;
	phone?: string;
	rsvpStatus: 'pending' | 'attending' | 'maybe' | 'declined';
	rsvpToken: string;
	contactMethod: 'email' | 'sms';
	dietaryNotes: string;
	plusOnes: number;
	createdAt: string;
	updatedAt: string;
}

export interface Message {
	id: string;
	eventId: string;
	senderType: 'organizer' | 'attendee';
	senderId: string;
	recipientType: 'organizer' | 'attendee' | 'group';
	recipientId: string;
	subject: string;
	body: string;
	readAt?: string;
	createdAt: string;
}

export interface Reminder {
	id: string;
	eventId: string;
	remindAt: string;
	targetGroup: 'all' | 'attending' | 'maybe' | 'declined' | 'pending';
	message: string;
	status: 'scheduled' | 'sent' | 'cancelled' | 'failed';
	createdAt: string;
	updatedAt: string;
}

export interface RSVPStats {
	attending: number;
	maybe: number;
	declined: number;
	pending: number;
	total: number;
}

export interface ApiError {
	error: string;
	message: string;
	status: number;
}

export interface ApiResponse<T> {
	data: T;
}

export interface PaginatedResponse<T> {
	data: T[];
	total: number;
	page: number;
	perPage: number;
}

export type InviteTemplate = {
	id: string;
	name: string;
	description: string;
	previewImage: string;
};
