package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGetFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.com", "index.html"},
		{"http://example.com/", "index.html"},
		{"http://example.com/?test=%gg*", "_test__gg_"},
		{"http://example.com/世界", "世界"},
		{"http://example.com/././fff/../../../test.htm", "test.htm"},
		{"http://example.com/././fff/../../..", "index.html"},
	}

	d := NewDownloader()
	for _, v := range cases {
		got := d.getFilename(v.in)
		if got != v.want {
			t.Errorf("getFilename(%q) == %q, want %q", v.in, got, v.want)
		}
	}
}

type mockPrinter struct {
	stdout string
	stderr string
}

func (mp *mockPrinter) Printf(format string, a ...interface{}) (n int, err error) {
	str := fmt.Sprintf(format, a...)
	mp.stdout += str
	return len(str), nil
}

func (mp *mockPrinter) ErrPrintf(format string, a ...interface{}) (n int, err error) {
	str := fmt.Sprintf(format, a...)
	mp.stderr += str
	return len(str), nil
}

// chdirTmp creates temporary dir, changes working dir there and return new wd and old wd
func chdirTmp() (tmpdir, oldPwd string, err error) {
	if tmpdir, err = ioutil.TempDir("", "gowgettest_"); err != nil {
		return "", "", err
	}

	oldPwd, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	os.Chdir(tmpdir)

	return tmpdir, oldPwd, nil
}

func restoreWorkingDir(tmpdir, oldPwd string) {
	os.Chdir(oldPwd)
	os.RemoveAll(tmpdir)
}

func TestInitializeStatusTable(t *testing.T) {
	// change working dir to ensure that there are no existing files with target names
	tmpdir, oldPwd, err := chdirTmp()
	if err != nil {
		t.Errorf("chdirTmp: %q", err.Error())
	}
	defer restoreWorkingDir(tmpdir, oldPwd)

	testUrls := []string{
		"http://example.com/0",
		"http://example.com/output.dat",
		"http://example.com/000",
	}
	wantedStatusTableRowFormat := "%3d%% %9d%% %3d%% \n"
	wantedTableHeader := "   0 output.dat  000 \n"

	d := NewDownloader()
	p := &mockPrinter{}
	d.printer = p
	d.initializeStatusTable(testUrls)

	if d.statusTableRowFormat != wantedStatusTableRowFormat {
		t.Errorf("statusTableRowFormat == %q, want %q", d.statusTableRowFormat, wantedStatusTableRowFormat)
	}

	if p.stdout != wantedTableHeader {
		t.Errorf("p.stdout == %q, want %q", p.stdout, wantedTableHeader)
	}
}

func TestGetUniqueFilename(t *testing.T) {
	tmpdir, oldPwd, err := chdirTmp()
	if err != nil {
		t.Errorf("chdirTmp: %q", err.Error())
	}
	defer restoreWorkingDir(tmpdir, oldPwd)

	for _, v := range []string{"/existing", "/existing_", "/existing_.1"} {
		if err = os.Mkdir(tmpdir+v, 0777); err != nil {
			t.Errorf("Mkdir: %q", err.Error())
		}
	}

	cases := []struct{ in, want string }{
		{"index.html", "index.html"},
		{"existing", "existing.1"},
		{"existing_", "existing_.2"},
	}

	d := NewDownloader()
	for _, v := range cases {
		got := d.getUniqueFilename(v.in)
		if got != v.want {
			t.Errorf("getUniqueFilename(%q) == %q, want %q", v.in, got, v.want)
		}
	}
}
