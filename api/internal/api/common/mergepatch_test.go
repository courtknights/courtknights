package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mergePatchContext(contentType, body string) echo.Context {
	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}
	return echo.New().NewContext(req, httptest.NewRecorder())
}

func TestDecodeMergePatch_MediaTypes(t *testing.T) {
	tests := []struct {
		contentType string
		wantStatus  int // 0 = accepted
	}{
		{contentType: "application/merge-patch+json"},
		{contentType: "application/json"},
		{contentType: "application/merge-patch+json; charset=utf-8"},
		{contentType: "application/json; charset=UTF-8"},
		{contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{contentType: "application/json-patch+json", wantStatus: http.StatusUnsupportedMediaType},
		{contentType: "", wantStatus: http.StatusUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			_, err := DecodeMergePatch(mergePatchContext(tt.contentType, `{"city":"Madrid"}`))
			if tt.wantStatus == 0 {
				require.NoError(t, err)
				return
			}
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tt.wantStatus, he.Code)
		})
	}
}

func TestDecodeMergePatch_Body(t *testing.T) {
	t.Run("empty object", func(t *testing.T) {
		members, err := DecodeMergePatch(mergePatchContext("application/merge-patch+json", `{}`))
		require.NoError(t, err)
		assert.Empty(t, members)
	})
	t.Run("null member is kept raw", func(t *testing.T) {
		members, err := DecodeMergePatch(mergePatchContext("application/merge-patch+json", `{"city": null, "country": "ES"}`))
		require.NoError(t, err)
		assert.True(t, IsJSONNull(members["city"]))
		assert.Equal(t, json.RawMessage(`"ES"`), members["country"])
	})
	for _, body := range []string{``, `[]`, `"x"`, `null`, `42`, `{"a":`, `{} {}`} {
		t.Run("rejects "+body, func(t *testing.T) {
			_, err := DecodeMergePatch(mergePatchContext("application/merge-patch+json", body))
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, http.StatusBadRequest, he.Code)
		})
	}
}

func TestIsJSONNull(t *testing.T) {
	assert.True(t, IsJSONNull(json.RawMessage(`null`)))
	assert.True(t, IsJSONNull(json.RawMessage(` null `)))
	assert.False(t, IsJSONNull(json.RawMessage(`"null"`)))
	assert.False(t, IsJSONNull(json.RawMessage(`{}`)))
}
