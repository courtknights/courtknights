package profile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/biter777/countries"
)

// ValidateLocation returns an error if country or region are not valid ISO codes,
// or if region is provided without country.
// Uses an embedded dataset — no external API call.
//
// Rules:
//   - Both nil: no error.
//   - country non-nil: must be a valid, non-empty ISO 3166-1 alpha-2 code (uppercase).
//   - region non-nil: country must also be non-nil; region must be a valid ISO 3166-2
//     code for that country.
func ValidateLocation(country, region *string) error {
	if country == nil && region == nil {
		return nil
	}

	if region != nil && country == nil {
		return errors.New("region requires country to be provided")
	}

	// Validate country: must be a non-empty, uppercase, valid ISO 3166-1 alpha-2 code.
	if *country == "" {
		return errors.New("country code must not be empty")
	}

	// The library looks up by name (which is case-insensitive) but we require uppercase
	// alpha-2 codes specifically. We reject codes that are not exactly 2 uppercase letters.
	if len(*country) != 2 || *country != strings.ToUpper(*country) {
		return fmt.Errorf("invalid country code %q: must be an uppercase ISO 3166-1 alpha-2 code", *country)
	}

	countryCode := countries.ByName(*country)
	if !countryCode.IsValid() {
		return fmt.Errorf("invalid country code %q: not a recognised ISO 3166-1 alpha-2 code", *country)
	}

	// Ensure the alpha-2 code matches exactly (prevents accepting alpha-3 or name lookups).
	if countryCode.Alpha2() != *country {
		return fmt.Errorf("invalid country code %q: not a recognised ISO 3166-1 alpha-2 code", *country)
	}

	if region == nil {
		return nil
	}

	// Validate region: must be non-empty and a valid ISO 3166-2 code for the given country.
	if *region == "" {
		return errors.New("region code must not be empty")
	}

	subdivisionCode := countries.SubdivisionCode(*region)
	if !subdivisionCode.IsValid() {
		return fmt.Errorf("invalid region code %q: not a recognised ISO 3166-2 code", *region)
	}

	if subdivisionCode.Country() != countryCode {
		return fmt.Errorf("region code %q does not belong to country %q", *region, *country)
	}

	return nil
}
