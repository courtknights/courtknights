package profile

// Preferences holds sport-specific settings stored as JSONB.
// All fields are optional. New fields can be added here without a schema migration.
type Preferences struct {
	// CourtSide is the preferred side of the court.
	CourtSide *CourtSide `json:"court_side,omitempty"`
	// Handedness is the player's dominant hand.
	Handedness *Handedness `json:"handedness,omitempty"`
}
