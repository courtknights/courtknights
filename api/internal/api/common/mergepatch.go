package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/labstack/echo/v4"
)

// MIMEApplicationMergePatchJSON is the media type of a JSON Merge Patch (RFC 7396).
const MIMEApplicationMergePatchJSON = "application/merge-patch+json"

// DecodeMergePatch reads a JSON Merge Patch request body (ADR-015) and returns
// its members undecoded, so the caller can tell an omitted member from an
// explicit null.
//
// The Content-Type must be application/merge-patch+json or application/json
// (parameters such as charset are ignored); otherwise it returns a 415
// *echo.HTTPError. The body must be a single JSON object; otherwise it
// returns a 400 *echo.HTTPError.
func DecodeMergePatch(c echo.Context) (map[string]json.RawMessage, error) {
	mediaType, _, err := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	if err != nil || (mediaType != MIMEApplicationMergePatchJSON && mediaType != echo.MIMEApplicationJSON) {
		return nil, echo.NewHTTPError(http.StatusUnsupportedMediaType,
			"content type must be "+MIMEApplicationMergePatchJSON+" or "+echo.MIMEApplicationJSON)
	}

	dec := json.NewDecoder(c.Request().Body)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request body must be a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request body must contain a single JSON object")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request body must be a JSON object")
	}

	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request body must be a JSON object")
	}
	return members, nil
}

// IsJSONNull reports whether a raw JSON member is the literal null.
func IsJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
