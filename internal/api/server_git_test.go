package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

func TestGitStageAndCommitHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":["first.txt"]}`)
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	var stageBody gitrepo.Status
	mustDecode(t, stageResponse, &stageBody)
	if !strings.Contains(stageBody.Porcelain, "A  first.txt") || !strings.Contains(stageBody.Porcelain, "?? second.txt") {
		t.Fatalf("unexpected staged status: %q", stageBody.Porcelain)
	}

	commitResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/commit", `{"message":"add first","paths":["first.txt"]}`)
	if commitResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commitBody gitrepo.Commit
	mustDecode(t, commitResponse, &commitBody)
	if commitBody.Hash == "" {
		t.Fatal("expected commit hash")
	}

	statusResponse := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/git/status", "")
	var statusBody gitrepo.Status
	mustDecode(t, statusResponse, &statusBody)
	if strings.Contains(statusBody.Porcelain, "first.txt") || !strings.Contains(statusBody.Porcelain, "?? second.txt") {
		t.Fatalf("expected only second file to remain changed, got %q", statusBody.Porcelain)
	}
}

func TestGitBranchHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	run(t, root, "git", "commit", "-m", "initial")
	defaultBranch := currentAPITestBranch(t, root)
	run(t, root, "git", "switch", "-c", "feature/status-bar")
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	branchesResponse := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/git/branches", "")
	if branchesResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", branchesResponse.Code, branchesResponse.Body.String())
	}
	var branchesBody gitrepo.BranchList
	mustDecode(t, branchesResponse, &branchesBody)
	if !containsAPIBranch(branchesBody.Branches, defaultBranch, false) || !containsAPIBranch(branchesBody.Branches, "feature/status-bar", true) {
		t.Fatalf("unexpected branches: %+v", branchesBody.Branches)
	}

	switchResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/branch", `{"branch":"`+defaultBranch+`"}`)
	if switchResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", switchResponse.Code, switchResponse.Body.String())
	}
	var statusBody gitrepo.Status
	mustDecode(t, switchResponse, &statusBody)
	if statusBody.Branch != defaultBranch {
		t.Fatalf("expected switched branch %q, got %q", defaultBranch, statusBody.Branch)
	}
}

func currentAPITestBranch(t *testing.T, root string) string {
	t.Helper()
	output, err := gitrepo.NewClient().Status(context.Background(), root, gitrepo.StatusOptions{})
	if err != nil {
		t.Fatalf("read git status: %v", err)
	}
	if output.Branch == "" {
		t.Fatal("expected current branch")
	}
	return output.Branch
}

func containsAPIBranch(branches []gitrepo.Branch, name string, current bool) bool {
	for _, branch := range branches {
		if branch.Name == name && branch.Current == current {
			return true
		}
	}
	return false
}

func TestGitUnstageAndDiscardHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	run(t, root, "git", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":["tracked.txt","new.txt"]}`)
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}

	unstageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/unstage", `{"paths":["new.txt"]}`)
	if unstageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", unstageResponse.Code, unstageResponse.Body.String())
	}
	var unstageBody gitrepo.Status
	mustDecode(t, unstageResponse, &unstageBody)
	if !strings.Contains(unstageBody.Porcelain, "M  tracked.txt") || !strings.Contains(unstageBody.Porcelain, "?? new.txt") {
		t.Fatalf("unexpected unstage status: %q", unstageBody.Porcelain)
	}

	discardResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/discard", `{"paths":["new.txt"]}`)
	if discardResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", discardResponse.Code, discardResponse.Body.String())
	}
	var discardBody gitrepo.Status
	mustDecode(t, discardResponse, &discardBody)
	if strings.Contains(discardBody.Porcelain, "new.txt") || !strings.Contains(discardBody.Porcelain, "M  tracked.txt") {
		t.Fatalf("unexpected discard status: %q", discardBody.Porcelain)
	}
}

func TestGitStagePatchHandler(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	run(t, root, "git", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	patch, err := gitrepo.NewClient().Diff(context.Background(), root, "tracked.txt")
	if err != nil {
		t.Fatalf("read git diff: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	body, err := json.Marshal(map[string]string{"patch": patch.Diff})
	if err != nil {
		t.Fatalf("marshal stage patch body: %v", err)
	}
	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage-patch", string(body))
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	var stageBody gitrepo.Status
	mustDecode(t, stageResponse, &stageBody)
	if !strings.Contains(stageBody.Porcelain, "M  tracked.txt") {
		t.Fatalf("expected patch to be staged, got %q", stageBody.Porcelain)
	}
}

func TestGitHandlersReturnValidationErrors(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":[]}`)
	if stageResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	var stageBody map[string]map[string]any
	mustDecode(t, stageResponse, &stageBody)
	if stageBody["error"]["code"] != "empty_path_list" {
		t.Fatalf("unexpected stage error: %+v", stageBody)
	}

	commitResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/commit", `{"message":"","paths":["note.txt"]}`)
	if commitResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commitBody map[string]map[string]any
	mustDecode(t, commitResponse, &commitBody)
	if commitBody["error"]["code"] != "empty_commit_message" {
		t.Fatalf("unexpected commit error: %+v", commitBody)
	}
}
