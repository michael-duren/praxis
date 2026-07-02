package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/store"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "praxis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	home := t.TempDir()
	return New(st, harness.All(home)), home
}

func post(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestDashboardRenders(t *testing.T) {
	s, _ := testServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	for _, want := range []string{"praxis", "Your skills", "Autonomy", "Enabled harnesses"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
}

func TestAddSkillAndAdjustRank(t *testing.T) {
	s, _ := testServer(t)
	h := s.Handler()

	w := post(t, h, "/skills", url.Values{"name": {"go"}, "category": {"language"}})
	if w.Code != http.StatusOK {
		t.Fatalf("add skill status = %d: %s", w.Code, w.Body)
	}

	w = post(t, h, "/skills/go/rank?delta=1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("rank status = %d", w.Code)
	}
	skills, _ := s.st.Skills()
	if len(skills) != 1 || skills[0].Rank != domain.RankBeginner {
		t.Errorf("skills = %+v, want go at beginner", skills)
	}

	if w := post(t, h, "/skills/nope/rank?delta=1", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown skill status = %d, want 404", w.Code)
	}
	if w := post(t, h, "/skills", url.Values{}); w.Code != http.StatusBadRequest {
		t.Errorf("empty name status = %d, want 400", w.Code)
	}
}

func TestAddContextValidation(t *testing.T) {
	s, _ := testServer(t)
	h := s.Handler()

	if w := post(t, h, "/context", url.Values{"title": {"x"}}); w.Code != http.StatusBadRequest {
		t.Errorf("missing body status = %d, want 400", w.Code)
	}
	w := post(t, h, "/context", url.Values{"title": {"Style"}, "body": {"Prefer stdlib."}})
	if w.Code != http.StatusOK {
		t.Fatalf("add context status = %d", w.Code)
	}
	entries, _ := s.st.ContextEntries()
	if len(entries) != 1 || !entries[0].Scope.IsGlobal() {
		t.Errorf("entries = %+v", entries)
	}
}

func TestHarnessToggleAndSync(t *testing.T) {
	s, home := testServer(t)
	h := s.Handler()

	post(t, h, "/skills", url.Values{"name": {"go"}})
	if w := post(t, h, "/harness/claude/toggle", nil); w.Code != http.StatusOK {
		t.Fatalf("toggle status = %d", w.Code)
	}

	w := post(t, h, "/sync", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("sync status = %d", w.Code)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("sync should write global CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(data), `Skill "go"`) {
		t.Errorf("synced doc missing skill policy:\n%s", data)
	}
}

func TestAgentSkillToggle(t *testing.T) {
	s, home := testServer(t)
	h := s.Handler()
	skillDir := filepath.Join(home, ".claude", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if w := post(t, h, "/agent-skill/claude/demo/toggle", nil); w.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", w.Code, w.Body)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill dir should have moved to skills.disabled")
	}

	// Toggle back on.
	if w := post(t, h, "/agent-skill/claude/demo/toggle", nil); w.Code != http.StatusOK {
		t.Fatalf("re-toggle status = %d", w.Code)
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("skill dir should be back: %v", err)
	}

	if w := post(t, h, "/agent-skill/claude/nope/toggle", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown skill status = %d, want 404", w.Code)
	}
	if w := post(t, h, "/agent-skill/copilot/demo/toggle", nil); w.Code != http.StatusNotFound {
		t.Errorf("copilot (no skills) status = %d, want 404", w.Code)
	}
}

func TestAutonomyCycle(t *testing.T) {
	s, _ := testServer(t)
	post(t, s.Handler(), "/autonomy/cycle", nil)
	set, _ := s.st.LoadSettings()
	if set.GlobalAutonomy != domain.AutonomyFull {
		t.Errorf("autonomy = %v, want full (guided → full)", set.GlobalAutonomy)
	}
}
