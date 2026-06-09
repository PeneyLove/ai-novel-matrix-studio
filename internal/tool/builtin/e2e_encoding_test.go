package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func gbkBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatalf("encode GBK: %v", err)
	}
	return b
}

func TestE2EGBKRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test_gbk.txt")
	if err := os.WriteFile(path, gbkBytes(t, "你好世界\n这是第二行\n包含函数的测试\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	readTL, _ := tool.LookupBuiltin("read_file")
	editTL, _ := tool.LookupBuiltin("edit_file")
	grepTL, _ := tool.LookupBuiltin("grep")

	args := func(m map[string]any) json.RawMessage {
		b, _ := json.Marshal(m)
		return json.RawMessage(b)
	}

	out, err := readTL.Execute(context.Background(), args(map[string]any{"path": path}))
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if !strings.Contains(out, "你好世界") || !strings.Contains(out, "这是第二�?) {
		t.Errorf("read_file did not decode GBK to readable Chinese:\n%s", out)
	}

	if raw, _ := os.ReadFile(path); utf8.Valid(raw) {
		t.Error("read_file rewrote GBK file as UTF-8 on disk")
	}

	if _, err := editTL.Execute(context.Background(), args(map[string]any{
		"path":       path,
		"old_string": "这是第二�?,
		"new_string": "这是新的�?,
	})); err != nil {
		t.Fatalf("edit_file: %v", err)
	}

	raw2, _ := os.ReadFile(path)
	if utf8.Valid(raw2) {
		t.Error("edit_file rewrote GBK file as UTF-8 on disk")
	}
	decoded, _, _ := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw2)
	if s := string(decoded); !strings.Contains(s, "这是新的�?) || strings.Contains(s, "这是第二�?) {
		t.Errorf("edit not applied to GBK file on disk: %q", s)
	}

	out2, err := readTL.Execute(context.Background(), args(map[string]any{"path": path}))
	if err != nil {
		t.Fatalf("read_file after edit: %v", err)
	}
	if !strings.Contains(out2, "这是新的�?) {
		t.Errorf("read_file after edit missing new text:\n%s", out2)
	}

	grepOut, err := grepTL.Execute(context.Background(), args(map[string]any{
		"pattern": "函数",
		"path":    path,
	}))
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(grepOut, "函数") {
		t.Errorf("grep did not match decoded GBK content:\n%s", grepOut)
	}
}
