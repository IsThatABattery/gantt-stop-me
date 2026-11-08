package runner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// entry is one tarball entry. body is a regular file's contents.
type entry struct {
	tar.Header
	body string
}

func file(name, body string) entry {
	return entry{tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644}, body}
}
func dir(name string) entry {
	return entry{tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}, ""}
}
func symlink(name, target string) entry {
	return entry{tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target, Mode: 0o777}, ""}
}
func hardlink(name, target string) entry {
	return entry{tar.Header{Name: name, Typeflag: tar.TypeLink, Linkname: target, Mode: 0o644}, ""}
}
func special(name string, typ byte) entry {
	return entry{tar.Header{Name: name, Typeflag: typ, Mode: 0o644}, ""}
}

// tarball is a .tar.gz of entries, after a pax global header like the one GitHub's tarballs start with.
func tarball(t testing.TB, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header",
		PAXRecords: map[string]string{"comment": "8d3f0c1e"}})
	for _, e := range entries {
		if err != nil {
			break
		}
		h := e.Header
		h.Size = int64(len(e.body))
		if err = tw.WriteHeader(&h); err == nil {
			_, err = tw.Write([]byte(e.body))
		}
	}
	if err == nil {
		err = tw.Close()
	}
	if err == nil {
		err = gz.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// sparse is a .tar.gz holding one PAX sparse file of size bytes, all of it a hole. tar.Writer drops GNU.sparse
// records, so the two headers are written by hand.
func sparse(t testing.TB, name string, size int64) []byte {
	t.Helper()
	header := func(name string, typ byte, size int) []byte {
		h := make([]byte, 512)
		copy(h, name)
		copy(h[100:], "0000644\x00")
		copy(h[124:], fmt.Sprintf("%011o\x00", size))
		h[156] = typ
		copy(h[257:], "ustar\x0000")
		copy(h[148:], "        ") // the checksum counts its own field as spaces
		sum := 0
		for _, b := range h {
			sum += int(b)
		}
		copy(h[148:], fmt.Sprintf("%06o\x00 ", sum))
		return h
	}
	// Version 0.1 keeps the map in the records, here no data at all.
	var records string
	for _, kv := range []string{"GNU.sparse.major=0", "GNU.sparse.minor=1", "GNU.sparse.numblocks=0",
		fmt.Sprintf("GNU.sparse.realsize=%d", size)} {
		rec := " " + kv + "\n"
		n := len(rec) + 2 // a record starts with its own length, two digits for these
		records += strconv.Itoa(n) + rec
	}
	var raw bytes.Buffer
	raw.Write(header("PaxHeaders/x", tar.TypeXHeader, len(records)))
	raw.WriteString(records)
	raw.Write(make([]byte, 512-len(records)%512))
	raw.Write(header(name, tar.TypeReg, 0))
	raw.Write(make([]byte, 1024)) // the end of the archive
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(raw.Bytes())
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	d := t.TempDir()
	err := extract(bytes.NewReader(tarball(t,
		dir("owner-repo-8d3f0c1/"),
		file("owner-repo-8d3f0c1/README", "read me"),
		entry{tar.Header{Name: "owner-repo-8d3f0c1/src/run.sh", Typeflag: tar.TypeReg, Mode: 0o755}, "#!/bin/sh"},
		symlink("owner-repo-8d3f0c1/src/readme", "../README"),
		symlink("owner-repo-8d3f0c1/docs", "src"),
		symlink("owner-repo-8d3f0c1/src/back", "../docs/readme"), // through a later link, still inside
		hardlink("owner-repo-8d3f0c1/README.copy", "owner-repo-8d3f0c1/README"),
	)), d)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{"README": "read me", "src/run.sh": "#!/bin/sh", "src/readme": "read me",
		"docs/readme": "read me", "src/back": "read me", "README.copy": "read me"} {
		if got, err := os.ReadFile(filepath.Join(d, p)); err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", p, got, err, want)
		}
	}
	if info, err := os.Stat(filepath.Join(d, "src/run.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("src/run.sh lost its executable bit: %v %v", info.Mode(), err)
	}
	if _, err := os.Lstat(filepath.Join(d, "pax_global_header")); !os.IsNotExist(err) {
		t.Errorf("the pax global header became a file: %v", err)
	}
}

func TestExtractRefuses(t *testing.T) {
	cases := []struct {
		name    string
		entries []entry
		want    string // in the error, naming the entry
	}{
		{"absolute path", []entry{file("/etc/cron.d/x", "")}, "/etc/cron.d/x: absolute path"},
		{"dot-dot segment", []entry{file("r/../../etc/passwd", "")}, `r/../../etc/passwd: ".." in path`},
		{"character device", []entry{special("r/tty", tar.TypeChar)}, "r/tty: device"},
		{"block device", []entry{special("r/sda", tar.TypeBlock)}, "r/sda: device"},
		{"FIFO", []entry{special("r/pipe", tar.TypeFifo)}, "r/pipe: device files and FIFOs"},
		{"absolute symlink", []entry{symlink("r/etc", "/etc")}, "r/etc: symlink to an absolute path"},
		{"symlink outside", []entry{symlink("r/src/escape", "../../outside")}, "src/escape: symlink points outside"},
		{"symlink outside through another", []entry{symlink("r/here", "."), symlink("r/escape", "here/..")},
			"escape: symlink points outside"},
		{"symlink loop", []entry{symlink("r/a", "b"), symlink("r/b", "a")}, "symlink points outside"},
		{"file through a symlink", []entry{symlink("r/up", "."), file("r/up/x", "")}, "r/up/x: written through or over a symlink"},
		{"file over a symlink", []entry{symlink("r/link", "x"), file("r/link", "")}, "r/link: written through or over a symlink"},
		{"hardlink outside", []entry{hardlink("r/shadow", "r/../../etc/shadow")}, "r/shadow: hardlink"},
		{"hardlink to an absolute path", []entry{hardlink("r/shadow", "/etc/shadow")}, "r/shadow: hardlink"},
		{"hardlink to a symlink", []entry{symlink("r/sub/l", "../x"), hardlink("r/l", "r/sub/l")}, "r/l: hardlink"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := extract(bytes.NewReader(tarball(t, c.entries...)), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("extract() = %v, want an error with %q", err, c.want)
			}
		})
	}

	over := map[string]struct {
		source []byte
		want   string
	}{
		"a file over the size limit": {tarball(t, file("r/big", strings.Repeat("x", 100<<10))), "r/big: source is over"},
		// Every header counts too, though the files are empty.
		"entries over the size limit": {tarball(t, slices.Repeat([]entry{file("r/empty", "")}, 200)...), "source is over"},
		// A few hundred bytes that archive/tar reads back as 1 MiB of zeros.
		"a sparse file over the size limit": {sparse(t, "r/zeros", 1<<20), "r/zeros: source is over"},
	}
	for name, c := range over {
		t.Run(name, func(t *testing.T) {
			defer func(n int64) { maxSource = n }(maxSource)
			maxSource = 64 << 10
			if err := extract(bytes.NewReader(c.source), t.TempDir()); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("extract() = %v, want %q", err, c.want)
			}
		})
	}
	t.Run("not gzip", func(t *testing.T) {
		if err := extract(strings.NewReader("plain"), t.TempDir()); err == nil || !strings.Contains(err.Error(), ".tar.gz") {
			t.Errorf("extract() = %v, want not a .tar.gz", err)
		}
	})
}
