package domain

import "testing"

func TestRankRoundTrip(t *testing.T) {
	for r := RankNovice; r <= RankExpert; r++ {
		got, err := ParseRank(r.String())
		if err != nil {
			t.Fatalf("ParseRank(%q): %v", r.String(), err)
		}
		if got != r {
			t.Errorf("round trip: got %v, want %v", got, r)
		}
	}
	if _, err := ParseRank("bogus"); err == nil {
		t.Error("ParseRank(bogus): want error, got nil")
	}
}

func TestAutonomyModeRoundTrip(t *testing.T) {
	for m := AutonomyManual; m <= AutonomyFull; m++ {
		got, err := ParseAutonomyMode(m.String())
		if err != nil {
			t.Fatalf("ParseAutonomyMode(%q): %v", m.String(), err)
		}
		if got != m {
			t.Errorf("round trip: got %v, want %v", got, m)
		}
	}
	if _, err := ParseAutonomyMode("bogus"); err == nil {
		t.Error("ParseAutonomyMode(bogus): want error, got nil")
	}
}

func TestScope(t *testing.T) {
	if !(Scope{}).IsGlobal() {
		t.Error("empty scope should be global")
	}
	if (Scope{Repo: "/x"}).IsGlobal() {
		t.Error("repo scope should not be global")
	}
	if got := (Scope{}).String(); got != "global" {
		t.Errorf("global scope String() = %q", got)
	}
}

func TestPolicyFor(t *testing.T) {
	tests := []struct {
		name string
		rank Rank
		mode AutonomyMode
		want Policy
	}{
		{
			name: "manual mode blocks edits even for experts",
			rank: RankExpert,
			mode: AutonomyManual,
			want: Policy{AllowEdits: false, UserTypes: true, Explain: true, Quiz: false},
		},
		{
			name: "manual mode quizzes weak skills",
			rank: RankNovice,
			mode: AutonomyManual,
			want: Policy{AllowEdits: false, UserTypes: true, Explain: true, Quiz: true},
		},
		{
			name: "guided mode makes beginners type",
			rank: RankBeginner,
			mode: AutonomyGuided,
			want: Policy{AllowEdits: false, UserTypes: true, Explain: true, Quiz: true},
		},
		{
			name: "guided mode lets intermediates delegate edits",
			rank: RankIntermediate,
			mode: AutonomyGuided,
			want: Policy{AllowEdits: true, UserTypes: false, Explain: true, Quiz: false},
		},
		{
			name: "guided mode stops explaining at expert",
			rank: RankExpert,
			mode: AutonomyGuided,
			want: Policy{AllowEdits: true, UserTypes: false, Explain: false, Quiz: false},
		},
		{
			name: "full mode allows edits but still explains to novices",
			rank: RankNovice,
			mode: AutonomyFull,
			want: Policy{AllowEdits: true, UserTypes: false, Explain: true, Quiz: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := UserSkill{Name: "go", Rank: tt.rank}
			got := PolicyFor(skill, tt.mode)
			if got.AllowEdits != tt.want.AllowEdits {
				t.Errorf("AllowEdits = %v, want %v", got.AllowEdits, tt.want.AllowEdits)
			}
			if got.UserTypes != tt.want.UserTypes {
				t.Errorf("UserTypes = %v, want %v", got.UserTypes, tt.want.UserTypes)
			}
			if got.Explain != tt.want.Explain {
				t.Errorf("Explain = %v, want %v", got.Explain, tt.want.Explain)
			}
			if got.Quiz != tt.want.Quiz {
				t.Errorf("Quiz = %v, want %v", got.Quiz, tt.want.Quiz)
			}
			if got.Description == "" {
				t.Error("Description is empty")
			}
		})
	}
}
