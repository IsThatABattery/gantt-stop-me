// Package githubtest fakes the parts of GitHub the API calls, for tests.
package githubtest

import (
	"archive/tar"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"regexp"
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
	lastID   int64 // of repositories, issues, labels and comments
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
	files  map[string]map[string]string // commit sha -> path -> content
	merges []string                     // the merge methods it allows
}

// Issue is an issue or a pull request in a Repo.
type Issue struct {
	ID          int64
	Number      int
	Title, Body string
	State       string // "open" or "closed"
	Labels      []string
	Pull        *Pull // set on pull requests only
	Comments    []Comment
	UpdatedAt   time.Time
	Author      string   // login of an issue's author: the user who opened it through the API, or one a test sets
	Assignees   []string // logins
}

// Pull is what a pull request has beyond the issue GitHub's issue list shows for it.
type Pull struct {
	Author   string // login of a user AddUser made
	HeadSHA  string
	HeadRepo int64 // id of the repository its branch is in, 0 once that fork is deleted
	Draft    bool
	Closes   []Ref // the issues it closes, such as with "Closes #42"
	// Unmergeable is GitHub's message refusing to merge it, as for a conflict or a required check; "" lets it merge.
	Unmergeable string
	// MergeState is GitHub's mergeable_state for it, such as "blocked"; "" is "clean". The merge route ignores it, as
	// GitHub does for an admin who may bypass branch protection.
	MergeState string
	MergedWith string // the merge method that merged it, "" until then
}

// Ref names an issue in a repository.
type Ref struct {
	RepoID int64
	Number int
}

// Comment is a comment on an issue or pull request.
type Comment struct {
	Author string // login
	Body   string
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
	f.mux.HandleFunc("POST /repositories/{id}/issues/{number}/comments", f.createComment)
	f.mux.HandleFunc("POST /repositories/{id}/issues/{number}/assignees", f.addAssignees)
	f.mux.HandleFunc("GET /repositories/{id}/pulls", f.listPulls)
	f.mux.HandleFunc("GET /repositories/{id}/pulls/{number}", f.getPull)
	f.mux.HandleFunc("PUT /repositories/{id}/pulls/{number}/merge", f.mergePull)
	f.mux.HandleFunc("GET /repositories/{id}/collaborators/{login}/permission", f.permission)
	f.mux.HandleFunc("GET /repositories/{id}/contents/{path...}", f.contents)
	f.mux.HandleFunc("GET /repositories/{id}/tarball/{sha}", f.tarball)
	f.mux.HandleFunc("GET "+codeload+"/{owner}/{name}/legacy.tar.gz/{sha}", f.codeload)
	f.mux.HandleFunc("POST /graphql", f.graphql)
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
	r := &Repo{ID: f.lastID, Owner: owner, Name: name, f: f, roles: map[string]string{}, merges: []string{"squash", "merge", "rebase"}}
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

// AllowMerges sets the merge methods the repository allows, of "squash", "merge" and "rebase", as its settings on
// GitHub would. A new repository allows all three.
func (r *Repo) AllowMerges(methods ...string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	r.merges = methods
}

// AddFile puts a file with content at name, a slash-separated path, in commit sha, for the contents API and the
// commit's tarball. Every commit exists, with the files AddFile gave it, so one given none has an empty tarball.
func (r *Repo) AddFile(sha, name, content string) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	if r.files == nil {
		r.files = map[string]map[string]string{}
	}
	if r.files[sha] == nil {
		r.files[sha] = map[string]string{}
	}
	r.files[sha][name] = content
}

// AddIssue opens an issue with these labels and returns its number.
func (r *Repo) AddIssue(title string, labels ...string) int {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	return r.add(title, "", labels, false).Number
}

// AddPullRequest opens a pull request, which GitHub's issue list mixes in with the issues, and returns its number.
func (r *Repo) AddPullRequest(title string) int {
	return r.AddPull(title, Pull{})
}

// AddPull opens a pull request with pull's fields and returns its number.
func (r *Repo) AddPull(title string, pull Pull) int {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	is := r.add(title, "", nil, true)
	is.Pull = &pull
	return is.Number
}

// EditPull changes pull request number the way its author or GitHub would. Close it with EditIssue.
func (r *Repo) EditPull(number int, edit func(*Pull)) {
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	is := r.issue(number)
	is.UpdatedAt = time.Now()
	edit(is.Pull)
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
	is.Comments = slices.Clone(is.Comments)
	is.Assignees = slices.Clone(is.Assignees)
	if is.Pull != nil {
		pull := *is.Pull
		pull.Closes = slices.Clone(pull.Closes)
		is.Pull = &pull
	}
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
	is := &Issue{ID: r.f.lastID, Number: len(r.issues) + 1, Title: title, Body: body, State: "open", UpdatedAt: time.Now()}
	if pr {
		is.Pull = &Pull{}
	}
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
	author := is.Author
	if is.Pull != nil {
		author = is.Pull.Author
	}
	user, _ := r.f.byLogin(author)
	assignees := []map[string]any{}
	for _, login := range is.Assignees {
		u, _ := r.f.byLogin(login)
		assignees = append(assignees, map[string]any{"id": u.ID, "login": login, "avatar_url": avatarURL(login)})
	}
	out := map[string]any{
		"id": is.ID, "node_id": nodeID(is), "number": is.Number, "title": is.Title, "body": body, "state": is.State,
		"labels": labels, "updated_at": is.UpdatedAt.UTC().Format(time.RFC3339), "user": user, "assignees": assignees,
	}
	if is.Pull != nil {
		out["pull_request"] = map[string]any{}
	}
	return out
}

// avatarURL is the fake's avatar address for login.
func avatarURL(login string) string { return "https://avatars.example.com/" + login }

// pullJSON renders a pull request the way GitHub's REST API does.
func (r *Repo) pullJSON(is *Issue) map[string]any {
	var headRepo any // null once the fork is deleted
	if is.Pull.HeadRepo != 0 {
		headRepo = map[string]int64{"id": is.Pull.HeadRepo}
	}
	author, _ := r.f.byLogin(is.Pull.Author)
	return map[string]any{
		"number": is.Number, "node_id": nodeID(is), "title": is.Title, "state": is.State, "draft": is.Pull.Draft,
		"head": map[string]any{"sha": is.Pull.HeadSHA, "repo": headRepo}, "user": author,
		// Finer than GitHub's seconds, so every edit changes the list's ETag, as it does on GitHub a second later.
		"updated_at": is.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// nodeID is the GraphQL id of an issue or pull request.
func nodeID(is *Issue) string {
	if is.Pull != nil {
		return fmt.Sprintf("PR_%d", is.ID)
	}
	return fmt.Sprintf("I_%d", is.ID)
}

// byLogin returns the user AddUser made with login and their token, or a zero user for a login nobody has.
func (f *Fake) byLogin(login string) (user, string) {
	for token, u := range f.users {
		if strings.EqualFold(u.Login, login) {
			return u, token
		}
	}
	return user{}, ""
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
	is := rp.add(in.Title, in.Body, in.Labels, false)
	is.Author = f.users[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")].Login
	writeJSON(w, http.StatusCreated, rp.json(is))
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

func (f *Fake) createComment(w http.ResponseWriter, r *http.Request) {
	_, is, ok := f.issue(w, r, false)
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	author := f.users[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	is.Comments = append(is.Comments, Comment{Author: author.Login, Body: in.Body})
	is.UpdatedAt = time.Now()
	f.lastID++
	writeJSON(w, http.StatusCreated, map[string]any{"id": f.lastID, "body": in.Body, "user": author})
}

// addAssignees serves POST /repositories/{id}/issues/{number}/assignees, which needs push access, like GitHub.
func (f *Fake) addAssignees(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Assignees []string `json:"assignees"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	for _, login := range in.Assignees {
		if !slices.Contains(is.Assignees, login) {
			is.Assignees = append(is.Assignees, login)
		}
	}
	is.UpdatedAt = time.Now()
	writeJSON(w, http.StatusCreated, rp.json(is))
}

// listPulls serves GET /repositories/{id}/pulls with its state, per_page and page parameters, newest first like
// GitHub's default order, an ETag per page, and a Link header to the next and last pages.
func (f *Fake) listPulls(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, false)
	if !ok {
		return
	}
	state := cmp.Or(r.URL.Query().Get("state"), "open")
	var items []map[string]any
	for _, is := range slices.Backward(rp.issues) {
		if is.Pull != nil && (state == "all" || state == is.State) {
			items = append(items, rp.pullJSON(is))
		}
	}
	writePage(w, r, items, f.PageSize)
}

func (f *Fake) getPull(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, false)
	if !ok {
		return
	}
	if is.Pull == nil {
		notFound(w)
		return
	}
	pr := rp.pullJSON(is)
	pr["merged"], pr["mergeable_state"] = is.Pull.MergedWith != "", cmp.Or(is.Pull.MergeState, "clean") // not in the list, as on GitHub
	writeJSON(w, http.StatusOK, pr)
}

// mergePull serves PUT /repositories/{id}/pulls/{number}/merge like GitHub: 405 for a pull request closed already,
// one Unmergeable, or a merge_method the repository does not allow, 409 when sha is set and is not its head, and
// else it closes the pull request as merged with merge_method, "merge" when unset.
func (f *Fake) mergePull(w http.ResponseWriter, r *http.Request) {
	rp, is, ok := f.issue(w, r, true)
	if !ok {
		return
	}
	if is.Pull == nil {
		notFound(w)
		return
	}
	var in struct {
		SHA    string `json:"sha"`
		Method string `json:"merge_method"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	method := cmp.Or(in.Method, "merge")
	switch {
	case is.State != "open":
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Pull Request is not mergeable"})
	case is.Pull.Unmergeable != "":
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": is.Pull.Unmergeable})
	case !slices.Contains(rp.merges, method):
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "The " + method + " merge method is not allowed on this repository."})
	case in.SHA != "" && in.SHA != is.Pull.HeadSHA:
		writeJSON(w, http.StatusConflict, map[string]string{"message": "Head branch was modified. Review and try the merge again."})
	default:
		is.State, is.Pull.MergedWith, is.UpdatedAt = "closed", method, time.Now()
		writeJSON(w, http.StatusOK, map[string]any{"sha": "merge-" + is.Pull.HeadSHA, "merged": true, "message": "Pull Request successfully merged"})
	}
}

// permission serves GET /repositories/{id}/collaborators/{login}/permission with GitHub's permission field for the
// role granted to login's token, or 404 for a login with no role.
func (f *Fake) permission(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, false)
	if !ok {
		return
	}
	u, token := f.byLogin(r.PathValue("login"))
	role, granted := rp.roles[token]
	if token == "" || !granted {
		notFound(w)
		return
	}
	// Like GitHub's permission field, which predates the maintain and triage roles.
	perm := map[string]string{"none": "none", "pull": "read", "triage": "read", "push": "write", "maintain": "write", "admin": "admin"}[role]
	writeJSON(w, http.StatusOK, map[string]any{"permission": perm, "user": u})
}

// contents serves GET /repositories/{id}/contents/{path...} at the commit its ref parameter names: the file itself
// when asked for application/vnd.github.raw+json, else GitHub's JSON with the content in base64, or 404 for a file
// the commit lacks. The fake has no default branch, so a request without a ref finds nothing.
func (f *Fake) contents(w http.ResponseWriter, r *http.Request) {
	rp, _, ok := f.repo(w, r, false)
	if !ok {
		return
	}
	p := r.PathValue("path")
	content, found := rp.files[r.URL.Query().Get("ref")][p]
	if !found {
		notFound(w)
		return
	}
	if r.Header.Get("Accept") == "application/vnd.github.raw+json" {
		w.Write([]byte(content))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"type": "file", "name": path.Base(p), "path": p, "size": len(content),
		"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
}

// codeload stands in for codeload.github.com, where GitHub redirects tarball downloads. Server.Client() sends every
// request for a host under example.com to the fake, and Go's client drops Authorization on the way to another host.
const codeload = "codeload.example.com"

// tarball serves GET /repositories/{id}/tarball/{sha} like GitHub: a redirect to the commit's archive on codeload.
func (f *Fake) tarball(w http.ResponseWriter, r *http.Request) {
	if rp, _, ok := f.repo(w, r, false); ok {
		http.Redirect(w, r, "http://"+codeload+"/"+url.PathEscape(rp.Owner)+"/"+url.PathEscape(rp.Name)+
			"/legacy.tar.gz/"+url.PathEscape(r.PathValue("sha")), http.StatusFound)
	}
}

// codeload serves a commit's archive like GitHub's: a .tar.gz of the files AddFile put at the commit, in one
// owner-repo-sha/ folder, after a pax global header holding the commit id. It refuses a request carrying
// Authorization, so a client that sends the token off the API fails its test.
// shortcut: it serves any repository to anyone, where GitHub's redirect carries a short-lived token; check one when
// a test needs codeload to refuse.
func (f *Fake) codeload(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "The GitHub API token reached codeload"})
		return
	}
	i := slices.IndexFunc(f.repos, func(rp *Repo) bool {
		return strings.EqualFold(rp.Owner, r.PathValue("owner")) && strings.EqualFold(rp.Name, r.PathValue("name"))
	})
	if i < 0 {
		notFound(w)
		return
	}
	rp, sha := f.repos[i], r.PathValue("sha")
	dir := fmt.Sprintf("%s-%s-%.7s/", rp.Owner, rp.Name, sha)
	w.Header().Set("Content-Type", "application/x-gzip")
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": sha}})
	tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: dir, Mode: 0o755})
	files := rp.files[sha]
	for _, p := range slices.Sorted(maps.Keys(files)) {
		tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: dir + p, Mode: 0o644, Size: int64(len(files[p]))})
		tw.Write([]byte(files[p]))
	}
	tw.Close()
	gz.Close()
}

// operation is a GraphQL document's operation name, by which graphql tells the API's operations apart.
var operation = regexp.MustCompile(`^\s*(?:query|mutation)\s+(\w+)`)

// graphql serves POST /graphql for the operations the API sends, recognized by name. Like GitHub, it answers a
// query it cannot serve with 200 and an "errors" list.
func (f *Fake) graphql(w http.ResponseWriter, r *http.Request) {
	if _, ok := f.auth(w, r); !ok {
		return
	}
	var in struct {
		Query     string                     `json:"query"`
		Variables map[string]json.RawMessage `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	name := ""
	if m := operation.FindStringSubmatch(in.Query); m != nil {
		name = m[1]
	}
	switch name {
	case "ClosingIssues":
		var ids []string
		json.Unmarshal(in.Variables["ids"], &ids)
		f.closingIssues(w, r, ids)
	case "LinkedPullRequests":
		var id string
		json.Unmarshal(in.Variables["id"], &id)
		f.linkedPullRequests(w, r, id)
	case "ConvertToDraft":
		var id string
		json.Unmarshal(in.Variables["id"], &id)
		f.convertToDraft(w, r, id)
	case "MergeSettings":
		var id string
		json.Unmarshal(in.Variables["id"], &id)
		f.mergeSettings(w, r, id)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"errors": []map[string]string{{"message": "Unknown operation " + name}}})
	}
}

// closingIssues answers ClosingIssues with each node id's pull request and the issues it closes, or null and a
// NOT_FOUND error for one the token cannot see.
func (f *Fake) closingIssues(w http.ResponseWriter, r *http.Request, ids []string) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	nodes := []any{}
	var errs []map[string]any
	for i, id := range ids {
		var node any
		if _, is, _ := f.pullByNode(token, id); is != nil {
			refs := []map[string]any{}
			for _, ref := range is.Pull.Closes {
				refs = append(refs, map[string]any{"number": ref.Number, "repository": map[string]int64{"databaseId": ref.RepoID}})
			}
			node = map[string]any{"number": is.Number, "closingIssuesReferences": map[string]any{"nodes": refs}}
		}
		if node == nil {
			errs = append(errs, map[string]any{"type": "NOT_FOUND", "path": []any{"nodes", i},
				"message": "Could not resolve to a node with the global id of '" + id + "'"})
		}
		nodes = append(nodes, node)
	}
	out := map[string]any{"data": map[string]any{"nodes": nodes}}
	if errs != nil {
		out["errors"] = errs
	}
	writeJSON(w, http.StatusOK, out)
}

// linkedPullRequests answers LinkedPullRequests with the pull requests, in repositories the token has a role on, that
// close the issue with node id, or with null and a NOT_FOUND error for an issue the token cannot see.
func (f *Fake) linkedPullRequests(w http.ResponseWriter, r *http.Request, id string) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var issue Ref
	for _, rp := range f.repos {
		for _, is := range rp.issues {
			if _, granted := rp.roles[token]; granted && is.Pull == nil && nodeID(is) == id {
				issue = Ref{rp.ID, is.Number}
			}
		}
	}
	if issue.Number == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"node": nil},
			"errors": []map[string]any{{"type": "NOT_FOUND", "path": []any{"node"}, "message": "Could not resolve to a node with the global id of '" + id + "'"}}})
		return
	}
	nodes := []map[string]any{}
	for _, rp := range f.repos {
		if _, granted := rp.roles[token]; !granted {
			continue
		}
		for _, is := range rp.issues {
			if is.Pull == nil || !slices.Contains(is.Pull.Closes, issue) {
				continue
			}
			state := map[bool]string{true: "OPEN", false: "CLOSED"}[is.State == "open"]
			if is.Pull.MergedWith != "" {
				state = "MERGED"
			}
			nodes = append(nodes, map[string]any{"number": is.Number, "title": is.Title, "state": state, "isDraft": is.Pull.Draft,
				"url": fmt.Sprintf("%s/%s/%s/pull/%d", f.Server.URL, rp.Owner, rp.Name, is.Number)})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"node": map[string]any{
		"closedByPullRequestsReferences": map[string]any{"nodes": nodes}}}})
}

// convertToDraft answers ConvertToDraft by making the pull request with node id a draft, or with null and an error
// for one the token cannot see or cannot push to.
func (f *Fake) convertToDraft(w http.ResponseWriter, r *http.Request, id string) {
	_, is, role := f.pullByNode(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), id)
	refuse := func(kind, message string) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"convertPullRequestToDraft": nil},
			"errors": []map[string]any{{"type": kind, "path": []any{"convertPullRequestToDraft"}, "message": message}}})
	}
	switch {
	case is == nil:
		refuse("NOT_FOUND", "Could not resolve to a node with the global id of '"+id+"'")
	case slices.Index(roles, role) < slices.Index(roles, "push"):
		refuse("FORBIDDEN", "Resource not accessible by integration")
	default:
		is.Pull.Draft = true
		is.UpdatedAt = time.Now()
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"convertPullRequestToDraft": map[string]any{"pullRequest": map[string]bool{"isDraft": true}}}})
	}
}

// mergeSettings answers MergeSettings with the merge methods the repository of the pull request with node id allows,
// or with null and a NOT_FOUND error for one the token cannot see.
func (f *Fake) mergeSettings(w http.ResponseWriter, r *http.Request, id string) {
	rp, is, _ := f.pullByNode(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), id)
	if is == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"node": nil},
			"errors": []map[string]any{{"type": "NOT_FOUND", "path": []any{"node"}, "message": "Could not resolve to a node with the global id of '" + id + "'"}}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"node": map[string]any{"repository": map[string]bool{
		"squashMergeAllowed": slices.Contains(rp.merges, "squash"),
		"mergeCommitAllowed": slices.Contains(rp.merges, "merge"),
		"rebaseMergeAllowed": slices.Contains(rp.merges, "rebase"),
	}}}})
}

// pullByNode returns the pull request with GraphQL node id in a repository token has a role on, with that repository
// and role, or nils.
func (f *Fake) pullByNode(token, id string) (*Repo, *Issue, string) {
	for _, rp := range f.repos {
		role, granted := rp.roles[token]
		if !granted {
			continue
		}
		for _, is := range rp.issues {
			if is.Pull != nil && nodeID(is) == id {
				return rp, is, role
			}
		}
	}
	return nil, nil, ""
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
