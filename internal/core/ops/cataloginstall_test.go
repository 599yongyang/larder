package ops

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestLocateSkillFolder(t *testing.T) {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"repo-main/skills/pdf/SKILL.md", "repo-main/skills/pdf/README.md", "repo-main/skills/other/SKILL.md"} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("x"))
	}
	_ = tw.Close()
	_ = gz.Close()
	dir, err := locateSkillFolder(buffer.Bytes(), "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if dir != "skills/pdf" {
		t.Fatalf("dir = %q", dir)
	}
}
