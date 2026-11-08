package runner

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// maxSource is the most uncompressed bytes a source tarball may hold. A variable so tests can lower it.
var maxSource int64 = 500 << 20

// extract reads a GitHub source tarball (.tar.gz) from r into dir, an empty directory.
// It drops the owner-repo-sha/ folder GitHub puts everything in, and refuses anything that could reach
// outside dir or onto the runner's devices. The error names the entry.
func extract(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("source is not a .tar.gz: %w", err)
	}
	root, err := os.OpenRoot(dir) // a second guard: writes through it cannot leave dir
	if err != nil {
		return err
	}
	defer root.Close()
	tr := tar.NewReader(&capped{r: gz, left: maxSource})
	links := map[string]string{} // symlink -> its target, as written
	files := map[string]bool{}   // regular files, the only entries a hardlink may name
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader { // GitHub's starts the tarball, holding the commit id
			continue
		}
		p, err := entryPath(hdr.Name)
		if err != nil {
			return fmt.Errorf("%s: %w", hdr.Name, err)
		}
		if p == "" {
			continue
		}
		// So every entry, and every symlink's directory, is where its name says. The symlink check below relies on it.
		if throughLink(links, p) {
			return fmt.Errorf("%s: written through or over a symlink", hdr.Name)
		}
		if hdr.Typeflag != tar.TypeDir {
			err = root.MkdirAll(path.Dir(p), 0o755)
		}
		switch {
		case err != nil:
		case hdr.Typeflag == tar.TypeDir:
			err = root.MkdirAll(p, 0o755)
		case hdr.Typeflag == tar.TypeReg:
			err = writeFile(root, p, hdr.FileInfo().Mode().Perm(), tr)
			files[p] = true
		case hdr.Typeflag == tar.TypeSymlink:
			if path.IsAbs(hdr.Linkname) {
				return fmt.Errorf("%s: symlink to an absolute path", hdr.Name)
			}
			links[p] = hdr.Linkname
			err = root.Symlink(hdr.Linkname, p)
		case hdr.Typeflag == tar.TypeLink:
			target, terr := entryPath(hdr.Linkname)
			if terr != nil || !files[target] {
				return fmt.Errorf("%s: hardlink to %s, which is not a file in the source", hdr.Name, hdr.Linkname)
			}
			err = root.Link(target, p)
		case hdr.Typeflag == tar.TypeChar || hdr.Typeflag == tar.TypeBlock || hdr.Typeflag == tar.TypeFifo:
			return fmt.Errorf("%s: device files and FIFOs are not allowed", hdr.Name)
		default:
			return fmt.Errorf("%s: unsupported entry type %q", hdr.Name, hdr.Typeflag)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", hdr.Name, err)
		}
	}
	// Checked once every link exists, since a link's target can pass through links that come later.
	for p := range links {
		if !inside(links, p) {
			return fmt.Errorf("%s: symlink points outside the source", p)
		}
	}
	return nil
}

// entryPath is where an entry named name goes, relative to the source directory, without its first segment.
// "" means the entry is the top folder itself.
func entryPath(name string) (string, error) {
	if path.IsAbs(name) {
		return "", errors.New("absolute path")
	}
	if strings.Contains("/"+name+"/", "/../") {
		return "", errors.New(`".." in path`)
	}
	_, rest, _ := strings.Cut(name, "/")
	if p := path.Clean(rest); p != "." {
		return p, nil
	}
	return "", nil
}

// throughLink reports whether p, or a directory above it, is a symlink from the tarball.
func throughLink(links map[string]string, p string) bool {
	for ; p != "."; p = path.Dir(p) {
		if _, ok := links[p]; ok {
			return true
		}
	}
	return false
}

// inside reports whether p, relative to the source directory, resolves inside it, following links the way
// the kernel would. A loop counts as outside.
func inside(links map[string]string, p string) bool {
	var at []string // the resolved directories so far
	todo := strings.Split(p, "/")
	for hops := 0; len(todo) > 0; {
		seg := todo[0]
		todo = todo[1:]
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(at) == 0 {
				return false
			}
			at = at[:len(at)-1]
			continue
		}
		at = append(at, seg)
		if target, ok := links[strings.Join(at, "/")]; ok {
			if hops++; hops > 255 {
				return false
			}
			at = at[:len(at)-1]
			todo = append(strings.Split(target, "/"), todo...)
		}
	}
	return true
}

// writeFile writes a regular file's contents from r.
func writeFile(root *os.Root, p string, perm os.FileMode, r io.Reader) error {
	f, err := root.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// capped fails a read once more than left bytes have come through, so an oversized source stops early.
type capped struct {
	r    io.Reader
	left int64
}

func (c *capped) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if c.left -= int64(n); c.left < 0 {
		return n, fmt.Errorf("source is over %d MiB uncompressed", maxSource>>20)
	}
	return n, err
}
