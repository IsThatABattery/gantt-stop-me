// Package githubtest fakes the parts of GitHub the API calls, for tests.
package githubtest

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fake serves GitHub's REST API and its OAuth endpoints from one test server, so a Client
// can point both its API and Web base URLs at Server.URL.
type Fake struct {
	Server       *httptest.Server
	ClientID     string // the only OAuth App credentials the token exchange accepts
	ClientSecret string
	PageSize     int // the most issues one page of the issue list holds, 100 like GitHub; lower it to test paging

	mux      *http.ServeMux
	mu       sync.Mutex        // held while serving each request
	users    map[string]user   // by token
	codes    map[string]string // OAuth code -> token
	repos    []*Repo
	failures map[string]int // route pattern -> the status it answers instead
	requests []string
	lastID   int64 // of repositories, issues and labels
}

type user struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// Repo is a repository on the fake GitHub. It is private: only tokens granted a role can see it.
type Repo struct {
	ID          int64
	Owner, Name string

	f      *Fake
	roles  map[string]string // token -> role
	labels []Label
	issues []*Issue
}

// Issue is an issue or a pull request in a Repo.
type Issue struct {
	ID          int64
	Number      int
	Title, Body string
	State       string // "open" or "closed"
	Labels      []string
	PullRequest bool
	UpdatedAt   time.Time
}

// Label is a repository's label.
type Label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// roles are GitHub's repository roles, weakest first. "none" sees the repository with no permission.
var roles = []string{"none", "pull", "triage", "push", "maintain", "admin"}

// New starts a Fake, closed when the test ends.
func New(t testing.TB) *Fake {
	f := &Fake{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		PageSize:     100,
		mux:          http.NewServeMux(),
		users:        map[string]user{},
		codes:        map[string]string{},
		failures:     map[string]int{},
	}
	f.mux.HandleFunc("GET /user", f.user)
	f.mux.HandleFunc("POST /login/oauth/access_token", f.accessToken)
	f.mux.HandleFunc("GET /repos/{owner}/{name}", f.getRepo)
	f.mux.HandleFunc("GET /repositories/{id}", f.getRepo)
	f.mux.HandleFunc("GET /repositories/{id}/labels", f.listLabels)
	f.mux.HandleFunc("POST /repositories/{id}/labels", f.createLabel)
	f.mux.HandleFunc("GET /repositories/{id}/issues", f.listIssues)
	f.mux.HandleFunc("POST /repositories/{id}/issues", f.createIssue)
	f.mux.HandleFunc("GET /repositories/{id}/issues/{number}", f.getIssue)
	f.mux.HandleFunc("PATCH /repositories/{id}/issues/{number}", f.editIssue)
	f.mux.HandleFunc("POST /repositories/{id}/issues/{number}/labels", f.addLabels)
	f.mux.HandleFunc("DELETE /repositories/{id}/issues/{number}/labels/{label}", f.removeLabel)
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// AddUser makes token authenticate as the GitHub account id and login.
func (f *Fake) AddUser(token string, id int64, login string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[token] = user{id, login}
}

// AddCode makes code a valid, single-use OAuth code that exchanges for token.
func (f *Fake) AddCode(code, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[code] = token
}

// Fail makes the route pattern, as New registers it (such as "POST /repositories/{id}/labels"),
// answer status instead of serving the request. Status 0 serves it again.
func (f *Fake) Fail(pattern string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[pattern] = status
}

// Requests returns every request served so far, as "METHOD /path?query".
func (f *Fake) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

// AddRepo creates the repository owner/name, which no token can see yet.
func (f *Fake) AddRepo(owner, name string) *Repo {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastID++
	r := &Repo{ID: f.lastID, Owner: owner, Name: name, f: f, roles: map[string]string{}}
	f.repos = append(f.repos, r)
	return r
}

// Grant gives token a role on the repository: "none", "pull", "triage", "push", "maintain" or "admin".
// Role "" takes the token's access away.
func (r *Repo) Grant(token, role string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	if role == "" {
		delete(r.roles, token)
		return
	}
	r.roles[token] = role
}

// Rename renames or transfers the repository, as its owner on GitHub would. Its id stays the same.
func (r *Repo) Rename(owner, name string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	r.Owner, r.Name = owner, name
}

// AddIssue opens an issue with these labels and returns its number.
func (r *Repo) AddIssue(title string, labels ...string) int {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	return r.add(title, "", labels, false).Number
}

// AddPullRequest opens a pull request, which GitHub's issue list mixes in with the issues, and returns its number.
func (r *Repo) AddPullRequest(title string) int {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	return r.add(title, "", nil, true).Number
}

// EditIssue changes issue number the way someone on GitHub would. UpdatedAt is set to now before edit runs,
// so edit may set it to something else.
func (r *Repo) EditIssue(number int, edit func(*Issue)) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	is := r.issue(number)
	is.UpdatedAt = time.Now()
	edit(is)
	for _, name := range is.Labels {
		r.label(name)
	}
}

// Issue returns a copy of issue number.
func (r *Repo) Issue(number int) Issue {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	is := *r.issue(number)
	is.Labels = slices.Clone(is.Labels)
	return is
}

// Labels returns the names of the repository's labels.
func (r *Repo) Labels() []string {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	var names []string
	for _, l := range r.labels {
		names = append(names, l.Name)
	}
	return names
}

// EditLabel changes the repository's label name, as someone would in its settings: the issues carrying it show the
// change, and their UpdatedAt stays as it is.
func (r *Repo) EditLabel(name string, edit func(*Label)) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	l := &r.labels[r.findLabel(name)]
	old := l.Name
	edit(l)
	for _, is := range r.issues {
		for i, n := range is.Labels {
			if strings.EqualFold(n, old) {
				is.Labels[i] = l.Name
			}
		}
	}
}

// DeleteLabel deletes the repository's label name, as someone would in its settings: it leaves every issue that
// carried it, and their UpdatedAt stays as it is.
func (r *Repo) DeleteLabel(name string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	i := r.findLabel(name)
	r.labels = slices.Delete(r.labels, i, i+1)
	for _, is := range r.issues {
		is.Labels = slices.DeleteFunc(is.Labels, func(n string) bool { return strings.EqualFold(n, name) })
	}
}

func (r *Repo) add(title, body string, labels []string, pr bool) *Issue {
	r.f.lastID++
	is := &Issue{ID: r.f.lastID, Number: len(r.issues) + 1, Title: title, Body: body, State: "open", PullRequest: pr, UpdatedAt: time.Now()}
	for _, name := range labels {
		is.Labels = append(is.Labels, r.label(name).Name)
	}
	r.issues = append(r.issues, is)
	return is
}

// issue returns issue number, or nil.
func (r *Repo) issue(number int) *Issue {
	if number < 1 || number > len(r.issues) {
		return nil
	}
	return r.issues[number-1]
}

// label returns the label named name, ignoring case like GitHub, creating it if it does not exist.
func (r *Repo) label(name string) *Label {
	if i := r.findLabel(name); i >= 0 {
		return &r.labels[i]
	}
	r.f.lastID++
	r.labels = append(r.labels, Label{ID: r.f.lastID, Name: name, Color: "ededed"})
	return &r.labels[len(r.labels)-1]
}

func (r *Repo) findLabel(name string) int {
	return slices.IndexFunc(r.labels, func(l Label) bool { return strings.EqualFold(l.Name, name) })
}

// json renders an issue the way GitHub's REST API does.
func (r *Repo) json(is *Issue) map[string]any {
	labels := []Label{}
	for _, name := range is.Labels {
		labels = append(labels, *r.label(name))
	}
	var body any
	if is.Body != "" {
		body = is.Body
	}
	out := map[string]any{
		"id": is.ID, "number": is.Number, "title": is.Title, "body": body, "state": is.State,
		"labels": labels, "updated_at": is.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if is.PullRequest {
		out["pull_request"] = map[string]any{}
	}
	return out
}

// serve logs the request and, unless its route is made to fail, serves it with mu held.
func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	if _, pattern := f.mux.Handler(r); f.failures[pattern] != 0 {
		writeJSON(w, f.failures[pattern], map[string]string{"message": "Injected failure"})
		return
	}
	f.mux.ServeHTTP(w, r)
}

// auth returns the user the request's bearer token belongs to. On failure it answers 401 like GitHub.
func (f *Fake) auth(w http.ResponseWriter, r *http.Request) (user, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	u, found := f.users[token]
	if !ok || !found {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return user{}, false
	}
	return u, true
}

// repo returns the repository the request's path names and the token's role on it. On failure it answers
// like GitHub: 401 for a bad token, 404 when the token cannot see the repository, and 403 when write is set
// and the token cannot push to it.
func (f *Fake) repo(w http.ResponseWriter, r *http.Request, write bool) (*Repo, string, bool) {
	if _, ok := f.auth(w, r); !ok {
		return nil, "", false
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	for _, rp := range f.repos {
		named := r.PathValue("id") == strconv.FormatInt(rp.ID, 10) ||
			strings.EqualFold(r.PathValue("owner"), rp.Owner) && strings.EqualFold(r.PathValue("name"), rp.Name)
		role, granted := rp.roles[token]
		if !named || !granted {
			continue
		}
		if write && slices.Index(roles, role) < slices.Index(roles, "push") {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Must have push access to repository"})
			return nil, "", false
		}
		return rp, role, true
	}
	notFound(w)
	return nil, "", false
}

// issue returns the repository and the issue the request's path names, answering like repo, or 404 for a missing issue.
func (f *Fake) issue(w http.ResponseWriter, r *http.Request, write bool) (*Repo, *Issue, bool) {
	rp, _, ok := f.repo(w, r, write)
	if !ok {
		return nil, nil, false
	}
	n, _ := strconv.Atoi(r.PathValue("number"))
	is := rp.issue(n)
	if is == nil {
		notFound(w)
		return nil, nil, false
	}
	return rp, is, true
}

func (f *Fake) user(w http.ResponseWriter, r *http.Request) {
	if u, ok := f.auth(w, r); ok {
		writeJSON(w, http.StatusOK, u)
	}
}

func (f *Fake) getRepo(w http.ResponseWriter, r *http.Request) {
	rp, role, ok := f.repo(w, r, false)
	if !ok {
		return
	}
	permissions := map[string]bool{}
	for i, name := range roles[1:] {
		permissions[name] = slices.Index(roles, role) > i
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": rp.ID, "name": rp.Name, "full_name": rp.Owner + "/" + rp.Name, "private": true,
		"owner": map[string]string{"login": rp.Owner}, "permissions": permissions,
	})
}

func (f *Fake) createLabel(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, true)
	if !ok {
		return
	}
	var in Label
	json.NewDecoder(r.Body).Decode(&in)
	if rp.findLabel(in.Name) >= 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Validation Failed",
			"errors":  []map[string]string{{"resource": "Label", "code": "already_exists", "field": "name"}},
		})
		return
	}
	f.lastID++
	in.ID = f.lastID
	rp.labels = append(rp.labels, in)
	writeJSON(w, http.StatusCreated, in)
}

// listLabels serves GET /repositories/{id}/labels with its per_page and page parameters, like listIssues.
func (f *Fake) listLabels(w http.ResponseWriter, r *http.Request) {
	if rp, _, ok := f.repo(w, r, false); ok {
		writePage(w, r, rp.labels, 100)
	}
}

// listIssues serves GET /repositories/{id}/issues with its state, sort, direction, per_page and page
// parameters, an ETag per page, and a Link header to the next and last pages.
func (f *Fake) listIssues(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, false)
	if !ok {
		return
	}
	q := r.URL.Query()
	state := cmp.Or(q.Get("state"), "open")
	var list []*Issue
	for _, is := range rp.issues {
		if state == "all" || state == is.State {
			list = append(list, is)
		}
	}
	slices.SortStableFunc(list, func(a, b *Issue) int {
		c := cmp.Compare(a.Number, b.Number)
		if q.Get("sort") == "updated" {
			c = a.UpdatedAt.Compare(b.UpdatedAt)
		}
		if q.Get("direction") != "asc" {
			c = -c
		}
		return c
	})
	var items []map[string]any
	for _, is := range list {
		items = append(items, rp.json(is))
	}
	writePage(w, r, items, f.PageSize)
}

// writePage answers a list request with the page of items its per_page and page parameters pick, at most limit
// to a page, an ETag per page, and a Link header to the next and last pages.
func writePage[T any](w http.ResponseWriter, r *http.Request, items []T, limit int) {
	q := r.URL.Query()
	size := min(atLeastOne(q.Get("per_page"), 30), limit)
	page := atLeastOne(q.Get("page"), 1)
	start := min((page-1)*size, len(items))
	end := min(start+size, len(items))
	body, _ := json.Marshal(append([]T{}, items[start:end]...)) // [], not null, for an empty page
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(body))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	if end < len(items) {
		link := func(page int) string {
			q.Set("page", strconv.Itoa(page))
			return "http://" + r.Host + r.URL.Path + "?" + q.Encode()
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next", <%s>; rel="last"`, link(page+1), link((len(items)+size-1)/size)))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func (f *Fake) createIssue(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	writeJSON(w, http.StatusCreated, rp.json(rp.add(in.Title, in.Body, in.Labels, false)))
}

func (f *Fake) getIssue(w http.ResponseWriter, r *http.Request) {
	if rp, is, ok := f.issue(w, r, false); ok {
		writeJSON(w, http.StatusOK, rp.json(is))
	}
}

func (f *Fake) editIssue(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Title *string `json:"title"`
		Body  *string `json:"body"`
		State *string `json:"state"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	if in.Title != nil {
		is.Title = *in.Title
	}
	if in.Body != nil {
		is.Body = *in.Body
	}
	if in.State != nil {
		is.State = *in.State
	}
	is.UpdatedAt = time.Now()
	writeJSON(w, http.StatusOK, rp.json(is))
}

func (f *Fake) addLabels(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Labels []string `json:"labels"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	for _, name := range in.Labels {
		l := rp.label(name)
		if !slices.Contains(is.Labels, l.Name) {
			is.Labels = append(is.Labels, l.Name)
		}
	}
	is.UpdatedAt = time.Now()
	writeJSON(w, http.StatusOK, rp.json(is)["labels"])
}

func (f *Fake) removeLabel(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, true)
	if !ok {
		return
	}
	i := slices.IndexFunc(is.Labels, func(name string) bool { return strings.EqualFold(name, r.PathValue("label")) })
	if i < 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "Label does not exist"})
		return
	}
	is.Labels = slices.Delete(is.Labels, i, i+1)
	is.UpdatedAt = time.Now()
	writeJSON(w, http.StatusOK, rp.json(is)["labels"])
}

func (f *Fake) accessToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Code         string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	token, found := f.codes[in.Code]
	delete(f.codes, in.Code)

	out := map[string]string{"access_token": token, "token_type": "bearer", "scope": "repo"}
	switch {
	case in.ClientID != f.ClientID || in.ClientSecret != f.ClientSecret:
		out = map[string]string{"error": "incorrect_client_credentials"}
	case !found:
		out = map[string]string{"error": "bad_verification_code"}
	}
	// Like GitHub: JSON only on request, otherwise form encoding, and 200 even for errors.
	if r.Header.Get("Accept") != "application/json" {
		form := url.Values{}
		for k, v := range out {
			form.Set(k, v)
		}
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte(form.Encode()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// atLeastOne parses a positive page parameter, or returns def.
func atLeastOne(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n >= 1 {
		return n
	}
	return def
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
