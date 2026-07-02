// Package web serves the praxis dashboard: templ-rendered HTML with HTMX
// interactions, backed by the same store and adapters as the TUI.
package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/michael-duren/praxis/internal/domain"
	"github.com/michael-duren/praxis/internal/harness"
	"github.com/michael-duren/praxis/internal/orchestrator"
	"github.com/michael-duren/praxis/internal/store"
	"github.com/michael-duren/praxis/internal/web/templates"
)

// Server holds the web app's dependencies.
type Server struct {
	st       *store.Store
	adapters []harness.Adapter
}

// New builds the praxis web server.
func New(st *store.Store, adapters []harness.Adapter) *Server {
	return &Server{st: st, adapters: adapters}
}

// Handler returns the routed http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleDashboard)
	mux.HandleFunc("POST /skills", s.handleAddSkill)
	mux.HandleFunc("POST /skills/{name}/rank", s.handleRank)
	mux.HandleFunc("POST /context", s.handleAddContext)
	mux.HandleFunc("POST /context/{id}", s.handleUpdateContext)
	mux.HandleFunc("POST /context/{id}/delete", s.handleDeleteContext)
	mux.HandleFunc("POST /autonomy/cycle", s.handleAutonomyCycle)
	mux.HandleFunc("POST /harness/{name}/toggle", s.handleHarnessToggle)
	mux.HandleFunc("POST /agent-skill/{harness}/{name}/toggle", s.handleAgentSkillToggle)
	mux.HandleFunc("POST /sync", s.handleSync)
	return mux
}

// ListenAndServe runs the server on addr.
func (s *Server) ListenAndServe(addr string) error {
	fmt.Printf("praxis web ui on http://%s\n", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// viewData loads everything the dashboard shows.
func (s *Server) viewData(status string) (templates.ViewData, error) {
	d := templates.ViewData{Status: status}
	var err error
	if d.Skills, err = s.st.Skills(); err != nil {
		return d, err
	}
	if d.Entries, err = s.st.ContextEntries(); err != nil {
		return d, err
	}
	if d.Settings, err = s.st.LoadSettings(); err != nil {
		return d, err
	}
	enabled, err := s.st.EnabledHarnesses()
	if err != nil {
		return d, err
	}
	for _, a := range s.adapters {
		d.Harnesses = append(d.Harnesses, templates.HarnessRow{
			Name: a.Name(), Enabled: enabled[a.Name()], Detected: a.Detect(),
		})
		if l, ok := a.(harness.SkillLister); ok {
			if skills, err := l.ListSkills(); err == nil {
				d.AgentSkills = append(d.AgentSkills, skills...)
			}
		}
	}
	return d, nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status string) {
	d, err := s.viewData(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := templates.Dashboard(d).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "")
}

func (s *Server) handleAddSkill(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	_, err := s.st.UpsertSkill(domain.UserSkill{
		Name: name, Category: r.FormValue("category"), Rank: domain.RankNovice,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "added skill "+name)
}

func (s *Server) handleRank(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	delta, err := strconv.Atoi(r.URL.Query().Get("delta"))
	if err != nil {
		http.Error(w, "bad delta", http.StatusBadRequest)
		return
	}
	skills, err := s.st.Skills()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, sk := range skills {
		if sk.Name != name {
			continue
		}
		rank := sk.Rank + domain.Rank(delta)
		if rank >= domain.RankNovice && rank <= domain.RankExpert {
			sk.Rank = rank
			if _, err := s.st.UpsertSkill(sk); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		s.render(w, r, fmt.Sprintf("%s → %s", sk.Name, sk.Rank))
		return
	}
	http.Error(w, "unknown skill", http.StatusNotFound)
}

func (s *Server) handleAddContext(w http.ResponseWriter, r *http.Request) {
	title, body := r.FormValue("title"), r.FormValue("body")
	if title == "" || body == "" {
		http.Error(w, "title and body are required", http.StatusBadRequest)
		return
	}
	_, err := s.st.UpsertContextEntry(domain.ContextEntry{
		Scope: domain.Scope{Repo: r.FormValue("repo")}, Title: title, Body: body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "added context "+title)
}

// contextID parses the {id} path value, writing a 400 on failure.
func contextID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad context entry id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// storeStatus maps store errors to HTTP codes.
func storeStatus(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func (s *Server) handleUpdateContext(w http.ResponseWriter, r *http.Request) {
	id, ok := contextID(w, r)
	if !ok {
		return
	}
	title, body := r.FormValue("title"), r.FormValue("body")
	if title == "" || body == "" {
		http.Error(w, "title and body are required", http.StatusBadRequest)
		return
	}
	_, err := s.st.UpsertContextEntry(domain.ContextEntry{
		ID: id, Scope: domain.Scope{Repo: r.FormValue("repo")}, Title: title, Body: body,
	})
	if err != nil {
		http.Error(w, err.Error(), storeStatus(err))
		return
	}
	s.render(w, r, "updated context "+title)
}

func (s *Server) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
	id, ok := contextID(w, r)
	if !ok {
		return
	}
	if err := s.st.DeleteContextEntry(id); err != nil {
		http.Error(w, err.Error(), storeStatus(err))
		return
	}
	s.render(w, r, "deleted context entry")
}

func (s *Server) handleAutonomyCycle(w http.ResponseWriter, r *http.Request) {
	set, err := s.st.LoadSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	set.GlobalAutonomy = (set.GlobalAutonomy + 1) % 3
	if err := s.st.SaveSettings(set); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "autonomy: "+set.GlobalAutonomy.String())
}

func (s *Server) handleHarnessToggle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	enabled, err := s.st.EnabledHarnesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.st.SetHarnessEnabled(name, !enabled[name]); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "toggled "+name)
}

func (s *Server) handleAgentSkillToggle(w http.ResponseWriter, r *http.Request) {
	harnessName, name := r.PathValue("harness"), r.PathValue("name")
	for _, a := range s.adapters {
		if a.Name() != harnessName {
			continue
		}
		lister, ok := a.(harness.SkillLister)
		if !ok {
			break // harness has no skills at all → 404 below
		}
		skills, err := lister.ListSkills()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, sk := range skills {
			if sk.Name != name {
				continue
			}
			tg, ok := a.(harness.SkillToggler)
			if !ok {
				http.Error(w, harnessName+" does not support toggling skills", http.StatusBadRequest)
				return
			}
			if err := tg.SetSkillEnabled(name, !sk.Enabled); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			state := "disabled"
			if !sk.Enabled {
				state = "enabled"
			}
			s.render(w, r, fmt.Sprintf("%s %s (%s)", name, state, harnessName))
			return
		}
	}
	http.Error(w, "unknown agent skill", http.StatusNotFound)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	skills, err := s.st.Skills()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entries, err := s.st.ContextEntries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	settings, err := s.st.LoadSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	enabled, err := s.st.EnabledHarnesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	st := orchestrator.State{Skills: skills, Entries: entries, Settings: settings}
	scopes := map[domain.Scope]bool{{}: true}
	for _, e := range entries {
		scopes[e.Scope] = true
	}
	var n int
	for scope := range scopes {
		for _, res := range orchestrator.Sync(st, scope, s.adapters, enabled) {
			if res.Err != nil {
				s.render(w, r, fmt.Sprintf("sync error (%s): %v", res.Harness, res.Err))
				return
			}
			n++
		}
	}
	s.render(w, r, fmt.Sprintf("synced %d file(s)", n))
}
