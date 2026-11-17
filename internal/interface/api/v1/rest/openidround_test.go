package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A sign-in through a provider that began before the update and finishes after
// it carries its round in the cookie the project's old name gave it. The round
// is still recognised, or somebody signing in at that moment would be sent back
// to start again for no reason they could see.
func TestARoundBegunBeforeTheRenameIsStillRecognised(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://timepulse.example.com/api/v1/auth/openid/finish", nil)
	r.AddCookie(&http.Cookie{Name: "gtr_openid", Value: "the-state"})

	if !sameRound(r, "the-state") {
		t.Error("a round begun under the old cookie name was not recognised")
	}

	if sameRound(r, "another-state") {
		t.Error("a round was recognised by a state it does not carry")
	}
}
