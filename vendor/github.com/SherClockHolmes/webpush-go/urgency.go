package webpush

// Urgency indicates to the push service how important a message is to the user.
// This can be used by the push service to help conserve the battery life of a user's device
// by only waking up for important messages when battery is low.
type Urgency string

const (
	// UrgencyVeryLow requires device state: on power and Wi-Fi
	UrgencyVeryLow Urgency = "very-low"
	// UrgencyLow requires device state: on either power or Wi-Fi
	UrgencyLow Urgency = "low"
	// UrgencyNormal excludes device state: low battery
	UrgencyNormal Urgency = "normal"
	// UrgencyHigh admits device state: low battery
	UrgencyHigh Urgency = "high"
)

// Checking allowable values for the urgency header
func isValidUrgency(urgency Urgency) bool {
	switch urgency {
	case UrgencyVeryLow, UrgencyLow, UrgencyNormal, UrgencyHigh:
		return true
	}
	return false
}
