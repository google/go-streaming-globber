// Copyright 2020 Google LLC

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

package glob

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// contains reports whether vector contains the string s.
func contains(vector []string, s string) bool {
	for _, elem := range vector {
		if elem == s {
			return true
		}
	}
	return false
}

func cmpStrings(x, y string) bool {
	return x < y
}

var sortStringSlices = cmpopts.SortSlices(cmpStrings)

func TestGlob(t *testing.T) {
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir("..")

	for _, tt := range []struct {
		pattern string
		results []string
	}{
		{"match", []string{"match"}},
		{"mat?h", []string{"match"}},
		{"../*/match", []string{"../testdata/match"}},
		{"*", []string{"a", "b", "match", "other"}},
		{"*/*", []string{"a/a", "a/b", "a/c", "b/a"}},
	} {
		pattern := tt.pattern
		results := tt.results
		if runtime.GOOS == "windows" {
			pattern = filepath.Clean(pattern)
			var cleaned []string
			for _, r := range results {
				cleaned = append(cleaned, r)
			}
			results = cleaned
		}
		matches, err := Glob(context.Background(), pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if diff := cmp.Diff(results, matches, sortStringSlices); diff != "" {
			t.Errorf("Bad results from Glob(%#q), -want +got: %v", pattern, diff)
		}
	}

	for _, pattern := range []string{"no_match", "../*/no_match"} {
		matches, err := Glob(context.Background(), pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if len(matches) != 0 {
			t.Errorf("Glob(%#q) = %#v want []", pattern, matches)
		}
	}
}

func TestGlobError(t *testing.T) {
	_, err := Glob(context.Background(), "[]")
	if err == nil {
		t.Error("expected error for bad pattern; got none")
	}
}

func TestGlobUNC(t *testing.T) {
	// Just make sure this runs without crashing for now.
	// See issue 15879.
	Glob(context.Background(), `\\?\C:\*`)
}

var globSymlinkTests = []struct {
	path, dest string
	brokenLink bool
}{
	{"test1", "link1", false},
	{"test2", "link2", true},
}

func TestGlobSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("skipping symlink test on Windows")
	}

	tmpDir, err := ioutil.TempDir("", "globsymlink")
	if err != nil {
		t.Fatal("creating temp dir:", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, tt := range globSymlinkTests {
		path := filepath.Join(tmpDir, tt.path)
		dest := filepath.Join(tmpDir, tt.dest)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		err = os.Symlink(path, dest)
		if err != nil {
			t.Fatal(err)
		}
		if tt.brokenLink {
			// Break the symlink.
			os.Remove(path)
		}
		matches, err := Glob(context.Background(), dest)
		if err != nil {
			t.Errorf("GlobSymlink error for %q: %s", dest, err)
		}
		if !contains(matches, dest) {
			t.Errorf("Glob(%#q) = %#v want %v", dest, matches, dest)
		}
	}
}

type globTest struct {
	pattern string
	matches []string
}

func (test *globTest) buildWant(root string) []string {
	want := make([]string, 0)
	for _, m := range test.matches {
		want = append(want, root+filepath.FromSlash(m))
	}
	sort.Strings(want)
	return want
}

func (test *globTest) globAbs(root, rootPattern string) error {
	p := filepath.FromSlash(rootPattern + `\` + test.pattern)
	have, err := Glob(context.Background(), p)
	if err != nil {
		return err
	}
	sort.Strings(have)
	want := test.buildWant(root + `\`)
	if strings.Join(want, "_") == strings.Join(have, "_") {
		return nil
	}
	return fmt.Errorf("Glob(%q) returns %q, but %q expected", p, have, want)
}

func (test *globTest) globRel(root string) error {
	p := root + filepath.FromSlash(test.pattern)
	have, err := Glob(context.Background(), p)
	if err != nil {
		return err
	}
	sort.Strings(have)
	want := test.buildWant(root)
	if strings.Join(want, "_") == strings.Join(have, "_") {
		return nil
	}
	// try also matching version without root prefix
	wantWithNoRoot := test.buildWant("")
	if strings.Join(wantWithNoRoot, "_") == strings.Join(have, "_") {
		return nil
	}
	return fmt.Errorf("Glob(%q) returns %q, but %q expected", p, have, want)
}

func TestWindowsGlob(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("skipping windows specific test")
	}

	tmpDir, err := ioutil.TempDir("", "TestWindowsGlob")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// /tmp may itself be a symlink
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal("eval symlink for tmp dir:", err)
	}

	if len(tmpDir) < 3 {
		t.Fatalf("tmpDir path %q is too short", tmpDir)
	}
	if tmpDir[1] != ':' {
		t.Fatalf("tmpDir path %q must have drive letter in it", tmpDir)
	}

	dirs := []string{
		"a",
		"b",
		"dir/d/bin",
	}
	files := []string{
		"dir/d/bin/git.exe",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0777)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		err := ioutil.WriteFile(filepath.Join(tmpDir, file), nil, 0666)
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := []globTest{
		{"a", []string{"a"}},
		{"b", []string{"b"}},
		{"c", []string{}},
		{"*", []string{"a", "b", "dir"}},
		{"d*", []string{"dir"}},
		{"*i*", []string{"dir"}},
		{"*r", []string{"dir"}},
		{"?ir", []string{"dir"}},
		{"?r", []string{}},
		{"d*/*/bin/git.exe", []string{"dir/d/bin/git.exe"}},
	}

	// test absolute paths
	for _, test := range tests {
		var p string
		err = test.globAbs(tmpDir, tmpDir)
		if err != nil {
			t.Error(err)
		}
		// test C:\*Documents and Settings\...
		p = tmpDir
		p = strings.Replace(p, `:\`, `:\*`, 1)
		err = test.globAbs(tmpDir, p)
		if err != nil {
			t.Error(err)
		}
		// test C:\Documents and Settings*\...
		p = tmpDir
		p = strings.Replace(p, `:\`, `:`, 1)
		p = strings.Replace(p, `\`, `*\`, 1)
		p = strings.Replace(p, `:`, `:\`, 1)
		err = test.globAbs(tmpDir, p)
		if err != nil {
			t.Error(err)
		}
	}

	// test relative paths
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.Chdir(wd)
		if err != nil {
			t.Fatal(err)
		}
	}()
	for _, test := range tests {
		err := test.globRel("")
		if err != nil {
			t.Error(err)
		}
		err = test.globRel(`.\`)
		if err != nil {
			t.Error(err)
		}
		err = test.globRel(tmpDir[:2]) // C:
		if err != nil {
			t.Error(err)
		}
	}
}

func TestNonWindowsGlobEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("skipping non-windows specific test")
	}
	pattern := `\glob.go`
	want := []string{"glob.go"}
	matches, err := Glob(context.Background(), pattern)
	if err != nil {
		t.Fatalf("Glob error for %q: %s", pattern, err)
	}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("Glob(%#q) = %v want %v", pattern, matches, want)
	}
}

func TestPartialGlob(t *testing.T) {
	gr := Stream("testdata/**")

	i := 0
	for i < 3 {
		match, err := gr.Next()
		i++
		if err != nil {
			t.Fatalf("Next() returned unexpected error: %v", err)
		}
		if match == "" {
			t.Fatalf("Next() unexpectedly stopped producing matches (returned \"\") after %d calls", i)
		}
	}

	err := gr.Close()
	if err != nil {
		t.Fatalf("Close() returned unexpected error: %v", err)
	}

	match, err := gr.Next()
	if err != nil {
		t.Errorf("After Close(), Next() returned unexpected error: %v", err)
	}
	if match != "" {
		t.Errorf("After Close(), Next() returned non-empty match %q", match)
	}
}

func TestCloseInvalidPattern(t *testing.T) {
	gr := Stream("[]") // This is an invalid glob pattern.
	err := gr.Close()
	if err != nil {
		t.Errorf("Close() on invalid patterns' result returned unexpected error: %v", err)
	}
}
