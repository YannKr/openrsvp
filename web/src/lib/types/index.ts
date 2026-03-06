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
	showHeadcount: boolean;
	showGuestList: boolean;
	status: 'draft' | 'published' | 'cancelled' | 'archived';
	shareToken: string;
	rsvpDeadline?: string;
	maxCapacity?: number;
	waitlistEnabled: boolean;
	seriesId?: string;
	seriesIndex?: number;
	seriesOverride?: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface EventSeries {
	id: string;
	organizerId: string;
	title: string;
	description: string;
	location: string;
	timezone: string;
	eventTime: string;
	durationMinutes?: number;
	recurrenceRule: 'weekly' | 'biweekly' | 'monthly';
	recurrenceEnd?: string;
	maxOccurrences?: number;
	seriesStatus: 'active' | 'stopped';
	retentionDays: number;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	showHeadcount: boolean;
	showGuestList: boolean;
	rsvpDeadlineOffsetHours?: number;
	maxCapacity?: number;
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
	rsvpStatus: 'pending' | 'attending' | 'maybe' | 'declined' | 'waitlisted';
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
	attendingHeadcount: number;
	maybe: number;
	maybeHeadcount: number;
	declined: number;
	pending: number;
	waitlisted: number;
	total: number;
	totalHeadcount: number;
}

export interface PublicEvent {
	title: string;
	description: string;
	eventDate: string;
	endDate?: string;
	location: string;
	timezone: string;
	contactRequirement: 'email' | 'phone' | 'email_or_phone' | 'email_and_phone';
	rsvpDeadline?: string;
	rsvpsClosed: boolean;
	maxCapacity?: number;
	spotsLeft?: number;
	atCapacity: boolean;
	waitlistEnabled: boolean;
}

export interface PublicAttendance {
	headcount: number;
	names?: string[];
}

export interface CoHost {
	id: string;
	eventId: string;
	organizerId: string;
	role: 'cohost';
	addedBy: string;
	organizerEmail: string;
	organizerName: string;
	createdAt: string;
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

export interface EventQuestion {
	id: string;
	eventId: string;
	label: string;
	type: 'text' | 'select' | 'checkbox';
	options: string[];
	required: boolean;
	sortOrder: number;
	createdAt: string;
	updatedAt: string;
}

export interface QuestionAnswer {
	id: string;
	attendeeId: string;
	questionId: string;
	answer: string;
	createdAt: string;
	updatedAt: string;
}

export type InviteTemplate = {
	id: string;
	name: string;
	description: string;
	previewImage: string;
};
