package tui

import "testing"

func TestThemesAreComplete(t *testing.T) {
	if len(Themes) < 5 {
		t.Fatalf("got %d themes, want at least 5", len(Themes))
	}
	seen := map[string]bool{}
	for _, th := range Themes {
		if th.Name == "" {
			t.Error("theme with empty name")
		}
		if seen[th.Name] {
			t.Errorf("duplicate theme %q", th.Name)
		}
		seen[th.Name] = true
		for field, c := range map[string]string{
			"Background": string(th.Background),
			"Foreground": string(th.Foreground),
			"Accent":     string(th.Accent),
			"Muted":      string(th.Muted),
			"Success":    string(th.Success),
			"Warning":    string(th.Warning),
		} {
			if len(c) != 7 || c[0] != '#' {
				t.Errorf("theme %q %s = %q, want #rrggbb", th.Name, field, c)
			}
		}
	}
	for _, want := range []string{"dracula", "tokyo-night", "light", "dark"} {
		if !seen[want] {
			t.Errorf("missing required theme %q", want)
		}
	}
}

func TestThemeByName(t *testing.T) {
	if got := ThemeByName("dracula").Name; got != "dracula" {
		t.Errorf("ThemeByName(dracula) = %q", got)
	}
	if got := ThemeByName("nope").Name; got != Themes[0].Name {
		t.Errorf("unknown theme should fall back to %q, got %q", Themes[0].Name, got)
	}
}
