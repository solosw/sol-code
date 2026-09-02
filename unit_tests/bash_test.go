package unit_tests

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/solosw/solcode/internal/tool"
)

func TestBashTool_IsDestructive(t *testing.T) {
	b := tool.NewBashTool()
	if !b.IsDestructive(nil) {
		t.Fatal("Bash should be destructive")
	}
	if b.IsReadOnly(nil) {
		t.Fatal("Bash should NOT be read-only")
	}
}

func TestTruncateOutput(t *testing.T) {
	long := strings.Repeat("x", 1000)
	short := tool.TruncateOutput(long, 100)
	if len(short) > 150 {
		t.Fatalf("expected truncated (<150), got len=%d", len(short))
	}
	if tool.TruncateOutput("hello", 100) != "hello" {
		t.Fatal("short content should be unchanged")
	}
}

func TestBashToolCancellationStopsCommandImmediately(t *testing.T) {
	command := "sleep 30"
	if runtime.GOOS == "windows" {
		command = "ping -n 30 127.0.0.1 > nul"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = tool.NewBashTool().Invoke(ctx, &tool.UseContext{}, json.RawMessage(`{"command":`+strconv.Quote(command)+`}`))
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bash invocation did not return promptly after cancellation")
	}
}

func TestBashWaitsForProcessExitDespiteContinuousOutput(t *testing.T) {
	// Training-like: prints early and keeps running. Bash must not finish on
	// first stdout — only when the process actually exits.
	dir := t.TempDir()
	scriptPath := dir + string(os.PathSeparator) + "train.py"
	script := "" +
		"import time\n" +
		"print('epoch 0 start', flush=True)\n" +
		"for i in range(1, 6):\n" +
		"    time.sleep(0.35)\n" +
		"    print(f'epoch {i}/5 loss={1.0/i:.4f}', flush=True)\n" +
		"print('training finished', flush=True)\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := "python " + scriptPath
	start := time.Now()
	res, err := tool.NewBashTool().Invoke(context.Background(), &tool.UseContext{}, json.RawMessage(
		`{"command":`+strconv.Quote(cmd)+`,"timeout":30000}`,
	))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("bash error: %s", res.Text)
	}
	if !strings.Contains(res.Text, "training finished") {
		t.Fatalf("expected full run output, got:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "epoch 5/5") {
		t.Fatalf("missing later epochs (returned too early on first output?):\n%s", res.Text)
	}
	// 5 * 0.35s ~= 1.75s; allow slack but fail if we returned on first print.
	if elapsed < 1500*time.Millisecond {
		t.Fatalf("returned too quickly (%s); likely finished on first stdout", elapsed)
	}
}

func TestBashAutoWaitStillWaitsForContinuousOutput(t *testing.T) {
	dir := t.TempDir()
	scriptPath := dir + string(os.PathSeparator) + "short.py"
	script := "import time\nprint('start', flush=True)\ntime.sleep(1.2)\nprint('end', flush=True)\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := "python " + scriptPath
	start := time.Now()
	res, err := tool.NewBashTool().Invoke(context.Background(), &tool.UseContext{}, json.RawMessage(
		`{"command":`+strconv.Quote(cmd)+`,"timeout":15000}`,
	))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("bash error: %s", res.Text)
	}
	if !strings.Contains(res.Text, "end") {
		t.Fatalf("expected end marker:\n%s", res.Text)
	}
	if elapsed < 1000*time.Millisecond {
		t.Fatalf("returned too quickly (%s)", elapsed)
	}
}
