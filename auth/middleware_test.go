package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
)

// protected wraps a sentinel handler in the same Verifier + Authenticator
// chain the app uses, and reports whether the sentinel ran.
func protected(ja *jwtauth.JWTAuth) (http.Handler, *bool) {
	called := false
	sentinel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return jwtauth.Verifier(ja)(Authenticator(ja)(sentinel)), &called
}

func TestAuthenticator(t *testing.T) {
	ja := jwtauth.New("HS256", []byte("secret-a"), nil)
	other := jwtauth.New("HS256", []byte("secret-b"), nil)

	token := func(signer *jwtauth.JWTAuth, claims map[string]interface{}) string {
		_, s, err := signer.Encode(claims)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	cases := []struct {
		name      string
		cookie    string
		wantPass  bool
		wantState int
	}{
		{"valid token", token(ja, map[string]interface{}{"user_id": 1}), true, http.StatusOK},
		{"no cookie", "", false, http.StatusFound},
		{"garbage", "not.a.jwt", false, http.StatusFound},
		{"wrong signing key", token(other, map[string]interface{}{"user_id": 1}), false, http.StatusFound},
		{"expired", token(ja, map[string]interface{}{"user_id": 1, "exp": time.Now().Add(-time.Hour).Unix()}), false, http.StatusFound},
	}
	for _, c := range cases {
		h, called := protected(ja)
		req := httptest.NewRequest("GET", "/", nil)
		if c.cookie != "" {
			req.AddCookie(&http.Cookie{Name: "jwt", Value: c.cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != c.wantState {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.wantState)
		}
		if *called != c.wantPass {
			t.Errorf("%s: next handler called = %v, want %v", c.name, *called, c.wantPass)
		}
		if !c.wantPass && rec.Header().Get("Location") != "/login" {
			t.Errorf("%s: redirect to %q, want /login", c.name, rec.Header().Get("Location"))
		}
	}
}

func TestGetCurrentUserID(t *testing.T) {
	ja := jwtauth.New("HS256", []byte("k"), nil)

	var got uint
	var gotErr error
	h := jwtauth.Verifier(ja)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, gotErr = GetCurrentUserID(r)
	}))

	_, tok, _ := ja.Encode(map[string]interface{}{"user_id": 42})
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tok})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if gotErr != nil || got != 42 {
		t.Errorf("got (%d, %v), want (42, nil)", got, gotErr)
	}

	// A valid token without a user_id claim must not yield user 0 silently.
	_, tok, _ = ja.Encode(map[string]interface{}{"foo": "bar"})
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tok})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if gotErr == nil {
		t.Error("expected error when user_id claim is missing")
	}

	// No token at all.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if gotErr == nil {
		t.Error("expected error with no token")
	}
}
