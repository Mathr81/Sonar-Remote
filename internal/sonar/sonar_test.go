package sonar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeSonar serves the two endpoints State reads and records every request.
func fakeSonar(t *testing.T, seen *[]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.Method+" "+r.URL.RequestURI())
		switch r.URL.Path {
		case "/volumeSettings/classic":
			fmt.Fprint(w, `{"masters":{"classic":{"volume":0.64,"muted":true}},
				"devices":{"game":{"classic":{"volume":0.8,"muted":false}},"chatCapture":{"classic":{"volume":1,"muted":false}}}}`)
		case "/v1/chatMix":
			fmt.Fprint(w, `{"balance":-0.5,"state":"enabled"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStateAndWrites(t *testing.T) {
	var seen []string
	srv := fakeSonar(t, &seen)
	c := NewClient()
	c.base, _ = url.Parse(srv.URL)

	st, err := c.State()
	if err != nil {
		t.Fatal(err)
	}
	want := State{ChatMix: -0.5, HasChatMix: true}
	want.Levels[0] = Level{0.64, true} // Master
	want.Levels[1] = Level{0.8, false} // game
	want.Levels[5] = Level{1, false}   // chatCapture
	if st != want {
		t.Errorf("State() = %+v, want %+v", st, want)
	}

	seen = nil
	c.SetVolume("game", 0.5)
	c.SetMute("Master", true)
	c.SetChatMix(0.25)
	wantCalls := []string{
		"PUT /volumeSettings/classic/game/Volume/0.5000",
		"PUT /volumeSettings/classic/Master/Mute/true",
		"PUT /v1/chatMix?balance=0.2500",
	}
	if !slices.Equal(seen, wantCalls) {
		t.Errorf("writes = %q, want %q", seen, wantCalls)
	}
}

// Sonar's port changes on every restart: a call against the stale address must
// rediscover it through GG and succeed.
func TestRediscoversAfterSonarRestart(t *testing.T) {
	var seen []string
	srv := fakeSonar(t, &seen)

	gg := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"subApps": map[string]any{"sonar": map[string]any{
			"metadata": map[string]any{"webServerAddress": srv.URL}}}})
	}))
	defer gg.Close()
	props := filepath.Join(t.TempDir(), "coreProps.json")
	os.WriteFile(props, []byte(`{"ggEncryptedAddress":"`+strings.TrimPrefix(gg.URL, "https://")+`"}`), 0o600)
	t.Setenv("SONAR_COREPROPS", props)

	c := NewClient()
	c.base, _ = url.Parse("http://127.0.0.1:1") // where Sonar used to be
	if _, err := c.State(); err != nil {
		t.Fatalf("State() after restart: %v", err)
	}
	if got := c.Base().String(); got != srv.URL {
		t.Errorf("base = %s, want %s", got, srv.URL)
	}
}
