package board_test

import (
	"net/http"
	"slices"
	"testing"
)

func TestAssignable(t *testing.T) {
	f := setup(t)
	bob := f.reader()
	res := f.Do("GET", f.path("/assignees"), nil, bob)
	res.Expect(http.StatusOK, "")
	var got struct {
		Assignees []struct{ Login, AvatarURL string }
	}
	res.JSON(&got)
	var logins []string
	for _, a := range got.Assignees {
		logins = append(logins, a.Login)
		if a.AvatarURL == "" {
			t.Fatalf("assignee %s has no avatar", a.Login)
		}
	}
	if !slices.Equal(logins, []string{"ada-gh", "bob-gh"}) {
		t.Fatalf("assignable = %v", logins)
	}
}

func TestEditTaskAssignees(t *testing.T) {
	f := setup(t, "One")
	f.reader()
	id, _ := f.task(1)
	type assigned struct{ Assignees []struct{ Login string } }
	assign := func(logins ...string) assigned {
		if logins == nil {
			logins = []string{} // [], not null: unassign everyone
		}
		res := f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{"assignees": logins}, f.ada)
		res.Expect(http.StatusOK, "")
		var got assigned
		res.JSON(&got)
		return got
	}

	// GitHub drops a login it cannot assign, and the answer shows who it kept.
	got := assign("bob-gh", "stranger")
	if len(got.Assignees) != 1 || got.Assignees[0].Login != "bob-gh" || !slices.Equal(f.repo.Issue(1).Assignees, []string{"bob-gh"}) {
		t.Fatalf("assignees = %+v, issue has %v", got.Assignees, f.repo.Issue(1).Assignees)
	}
	if got := assign(); len(got.Assignees) != 0 || len(f.repo.Issue(1).Assignees) != 0 {
		t.Fatalf("unassigning left %+v", got.Assignees)
	}

	for _, bad := range [][]string{{"-bad"}, {"ada-gh", "ada-gh"}, {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}} {
		f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{"assignees": bad}, f.ada).Expect(http.StatusBadRequest, "invalid_assignees")
	}
}

func TestAssignEnterpriseManagedUser(t *testing.T) {
	f := setup(t, "One")
	// An Enterprise Managed User's login is handle_shortcode.
	mona := f.SignUp("mona@example.com", "correct horse")
	f.repo.Grant(f.LinkGitHub(mona, "mona-cat_octo"), "push")
	id, _ := f.task(1)
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{"assignees": []string{"mona-cat_octo"}}, f.ada).Expect(http.StatusOK, "")
	if got := f.repo.Issue(1).Assignees; !slices.Equal(got, []string{"mona-cat_octo"}) {
		t.Fatalf("issue assignees = %v", got)
	}
}
