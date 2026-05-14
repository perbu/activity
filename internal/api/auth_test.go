package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantCalled bool
	}{
		{"missing header", "", http.StatusUnauthorized, false},
		{"malformed scheme", "Basic abc", http.StatusUnauthorized, false},
		{"empty token", "Bearer ", http.StatusUnauthorized, false},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized, false},
		{"first token", "Bearer goodtoken", http.StatusOK, true},
		{"second token", "Bearer alsogood", http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			mw := BearerAuth([]string{"goodtoken", "alsogood"})(next)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if called != tc.wantCalled {
				t.Errorf("next called = %v, want %v", called, tc.wantCalled)
			}
			if rec.Code == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got == "" {
					t.Errorf("missing WWW-Authenticate header on 401")
				}
			}
		})
	}
}
