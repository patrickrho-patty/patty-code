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

	"patty/internal/tool"
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
	if err := os.WriteFile(path, gbkBytes(t, "안녕 세상\n이것은 둘째 줄\n함수가 포함된 테스트\n"), 0o644); err != nil {
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
	if !strings.Contains(out, "안녕 세상") || !strings.Contains(out, "이것은 둘째 줄") {
		t.Errorf("read_file did not decode GBK to readable Korean:\n%s", out)
	}

	if raw, _ := os.ReadFile(path); utf8.Valid(raw) {
		t.Error("read_file rewrote GBK file as UTF-8 on disk")
	}

	if _, err := editTL.Execute(context.Background(), args(map[string]any{
		"path":       path,
		"old_string": "이것은 둘째 줄",
		"new_string": "이것은 새로운 줄",
	})); err != nil {
		t.Fatalf("edit_file: %v", err)
	}

	raw2, _ := os.ReadFile(path)
	if utf8.Valid(raw2) {
		t.Error("edit_file rewrote GBK file as UTF-8 on disk")
	}
	decoded, _, _ := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw2)
	if s := string(decoded); !strings.Contains(s, "이것은 새로운 줄") || strings.Contains(s, "이것은 둘째 줄") {
		t.Errorf("edit not applied to GBK file on disk: %q", s)
	}

	out2, err := readTL.Execute(context.Background(), args(map[string]any{"path": path}))
	if err != nil {
		t.Fatalf("read_file after edit: %v", err)
	}
	if !strings.Contains(out2, "이것은 새로운 줄") {
		t.Errorf("read_file after edit missing new text:\n%s", out2)
	}

	grepOut, err := grepTL.Execute(context.Background(), args(map[string]any{
		"pattern": "함수",
		"path":    path,
	}))
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(grepOut, "함수") {
		t.Errorf("grep did not match decoded GBK content:\n%s", grepOut)
	}
}

func e2eArgs(m map[string]any) json.RawMessage {
	b, _ := json.Marshal(m)
	return json.RawMessage(b)
}

func TestE2EWriteFilePreservesGBKOnOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gbk.txt")
	if err := os.WriteFile(path, gbkBytes(t, "원본 내용\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTL, _ := tool.LookupBuiltin("write_file")
	if _, err := writeTL.Execute(context.Background(), e2eArgs(map[string]any{
		"path":    path,
		"content": "완전히 새로운 내용\n둘째 줄\n",
	})); err != nil {
		t.Fatalf("write_file: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if utf8.Valid(raw) {
		t.Error("write_file rewrote an existing GBK file as UTF-8 on disk")
	}
	decoded, _, _ := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw)
	if s := string(decoded); !strings.Contains(s, "완전히 새로운 내용") || !strings.Contains(s, "둘째 줄") {
		t.Errorf("write_file content wrong after GBK round-trip: %q", s)
	}
}

func TestE2EWriteFileNewFileIsUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	writeTL, _ := tool.LookupBuiltin("write_file")
	if _, err := writeTL.Execute(context.Background(), e2eArgs(map[string]any{
		"path":    path,
		"content": "새 파일\n",
	})); err != nil {
		t.Fatalf("write_file: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !utf8.Valid(raw) {
		t.Error("a newly created file should be written as UTF-8")
	}
	if string(raw) != "새 파일\n" {
		t.Errorf("new file content = %q, want UTF-8 Korean", raw)
	}
}

func TestE2EDeleteRangePreservesGBK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gbk.txt")
	if err := os.WriteFile(path, gbkBytes(t, "첫째 줄\n둘째 줄\n셋째 줄\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delTL, _ := tool.LookupBuiltin("delete_range")
	if _, err := delTL.Execute(context.Background(), e2eArgs(map[string]any{
		"path":         path,
		"start_anchor": "둘째 줄",
		"end_anchor":   "둘째 줄",
	})); err != nil {
		t.Fatalf("delete_range on GBK file: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if utf8.Valid(raw) {
		t.Error("delete_range rewrote a GBK file as UTF-8 on disk")
	}
	decoded, _, _ := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw)
	s := string(decoded)
	if strings.Contains(s, "둘째 줄") {
		t.Errorf("delete_range did not remove the target line: %q", s)
	}
	if !strings.Contains(s, "첫째 줄") || !strings.Contains(s, "셋째 줄") {
		t.Errorf("delete_range removed the wrong lines: %q", s)
	}
}
