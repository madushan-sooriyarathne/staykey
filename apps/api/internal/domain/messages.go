package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Template is a message the team sends guests, with {placeholders} filled in when it is used.
type Template struct {
	ID        uuid.UUID
	Key       string
	Name      string
	Channels  []string // email, whatsapp
	Timing    string
	Body      string
	UpdatedAt time.Time
}

// DefaultTemplates are the four templates every account starts with. The owner app showed the
// same text before templates moved to the API.
func DefaultTemplates() []Template {
	return []Template{
		{
			Key: "confirmed", Name: "Booking confirmed", Channels: []string{"email"},
			Timing: "Right after confirmation",
			Body:   "Hi {guest_name}, your stay at {property_name} is confirmed for {check_in} to {check_out}. We can't wait to host you.",
		},
		{
			Key: "reminder", Name: "Payment reminder", Channels: []string{"email", "whatsapp"},
			Timing: "A day before it's due",
			Body:   "Hi {guest_name}, a friendly reminder that {balance} is due for your stay at {property_name}. You can pay here: {booking_link}",
		},
		{
			Key: "prearrival", Name: "Pre-arrival info", Channels: []string{"email", "whatsapp"},
			Timing: "2 days before check-in",
			Body:   "Hi {guest_name}, we look forward to welcoming you on {check_in}. Here is the map pin: {map_link}. Check-in is from {check_in_time}.",
		},
		{
			Key: "thanks", Name: "Thank you", Channels: []string{"email"},
			Timing: "The day after check-out",
			Body:   "Thank you for staying at {property_name}, {guest_name}. Book directly next time and we'll take care of the rest: {booking_link}",
		},
	}
}

// NormalizeTemplateBody trims a template body and checks its length.
func NormalizeTemplateBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 1000 {
		return "", &ValidationError{"body", "must be between 1 and 1000 characters"}
	}
	return body, nil
}

// NotificationPref is how one person wants to hear about one event in one account.
type NotificationPref struct {
	Event    NotificationEvent
	Push     bool
	WhatsApp bool
	// Email goes only to a confirmed address.
	Email bool
}

// DefaultNotificationPrefs are a new member's preferences.
func DefaultNotificationPrefs() []NotificationPref {
	return []NotificationPref{
		{EventBooking, true, true, false},
		{EventRequest, true, true, false},
		{EventPayment, true, false, false},
		{EventCancellation, true, false, false},
		{EventSync, true, false, false},
		{EventSummary, false, true, false},
	}
}
