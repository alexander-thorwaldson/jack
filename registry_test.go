//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestRegistryAdd(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "git@github.com:zoobzio/vicky.git")

	jtesting.AssertEqual(t, len(r.Projects), 1)
	jtesting.AssertEqual(t, r.Projects[0].Team, "blue")
	jtesting.AssertEqual(t, r.Projects[0].Repo, "vicky")
	jtesting.AssertEqual(t, r.Projects[0].URL, "git@github.com:zoobzio/vicky.git")
}

func TestRegistryAddReplacesExisting(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "git@github.com:zoobzio/vicky.git")
	r.Add("blue", "vicky", "git@github.com:zoobzio/vicky2.git")

	jtesting.AssertEqual(t, len(r.Projects), 1)
	jtesting.AssertEqual(t, r.Projects[0].URL, "git@github.com:zoobzio/vicky2.git")
}

func TestRegistryRemove(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "url1")
	r.Add("blue", "flux", "url2")
	r.Remove("blue", "vicky")

	jtesting.AssertEqual(t, len(r.Projects), 1)
	jtesting.AssertEqual(t, r.Projects[0].Repo, "flux")
}

func TestRegistryRemoveNonexistent(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "url1")
	r.Remove("blue", "nope")

	jtesting.AssertEqual(t, len(r.Projects), 1)
}

func TestRegistryFind(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "url1")
	r.Add("red", "flux", "url2")

	entry := r.Find("blue", "vicky")
	jtesting.AssertEqual(t, entry != nil, true)
	jtesting.AssertEqual(t, entry.URL, "url1")

	jtesting.AssertEqual(t, r.Find("blue", "nope") == nil, true)
}

func TestRegistryForTeam(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "url1")
	r.Add("blue", "alpha", "url2")
	r.Add("red", "flux", "url3")

	entries := r.ForTeam("blue")
	jtesting.AssertEqual(t, len(entries), 2)
	jtesting.AssertEqual(t, entries[0].Repo, "alpha")
	jtesting.AssertEqual(t, entries[1].Repo, "vicky")
}

func TestRegistryTeams(t *testing.T) {
	r := &Registry{}
	r.Add("red", "flux", "url1")
	r.Add("blue", "vicky", "url2")
	r.Add("blue", "alpha", "url3")

	teams := r.Teams()
	jtesting.AssertEqual(t, len(teams), 2)
	jtesting.AssertEqual(t, teams[0], "blue")
	jtesting.AssertEqual(t, teams[1], "red")
}

func TestRegistryReposForTeam(t *testing.T) {
	r := &Registry{}
	r.Add("blue", "vicky", "url1")
	r.Add("blue", "alpha", "url2")
	r.Add("red", "flux", "url3")

	repos := r.ReposForTeam("blue")
	jtesting.AssertEqual(t, len(repos), 2)
	jtesting.AssertEqual(t, repos[0], "alpha")
	jtesting.AssertEqual(t, repos[1], "vicky")

	jtesting.AssertEqual(t, len(r.ReposForTeam("green")), 0)
}

func TestRegistryLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	r := &Registry{}
	r.Add("blue", "vicky", "git@github.com:zoobzio/vicky.git")
	r.Add("red", "flux", "git@github.com:zoobzio/flux.git")

	err := saveRegistry(r)
	jtesting.AssertNoError(t, err)

	loaded, err := loadRegistry()
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(loaded.Projects), 2)
	jtesting.AssertEqual(t, loaded.Projects[0].Team, "blue")
	jtesting.AssertEqual(t, loaded.Projects[1].Team, "red")
}

func TestRegistryLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	r, err := loadRegistry()
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(r.Projects), 0)
}

func TestRegistrySaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	r := &Registry{}
	r.Add("blue", "vicky", "url")

	err := saveRegistry(r)
	jtesting.AssertNoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "registry.yaml"))
	jtesting.AssertNoError(t, err)
}
