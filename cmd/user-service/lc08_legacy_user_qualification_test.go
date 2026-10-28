package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// Only the exact copied legacy executable is started, in a network-none
// synthetic test namespace. No production or canonical data is used.
func TestLC08LegacyUserBinaryIdleShutdown(t *testing.T) {
	if os.Getenv("LC08_QUAL_NATS_URL") == "" {
		t.Skip("isolated broker opt-in required")
	}
	if runtime.GOOS != "linux" || os.Getenv("LC08_QUAL_ISOLATED") != "1" || os.Getenv("LC08_QUAL_NATS_URL") != "nats://127.0.0.1:4222" || os.Getenv("LC08_QUAL_PG_DSN") != "postgres://postgres@127.0.0.1:5432/lc08_producer_qualification_test?sslmode=disable" {
		t.Fatal("unexpected isolated qualification configuration")
	}
	if _, err := os.Readlink("/proc/self/ns/net"); err != nil {
		t.Fatal("network namespace unavailable")
	}
	if _, err := os.Stat("/.dockerenv"); err != nil {
		t.Fatal("container identity unavailable")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range interfaces {
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 {
			t.Fatal("non-loopback interface is up")
		}
	}
	path := "/tmp/lc08-legacy-user"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("exact legacy User binary not supplied")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "5596bb39c2ea5ba3e9a08169382d35fc6d78a7b826875417667b027dfd2f6f7d" {
		t.Fatal("legacy binary identity mismatch")
	}
	listen, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listen.Addr().(*net.TCPAddr).Port
	_ = listen.Close()
	child := exec.Command(path)
	dir, err := os.MkdirTemp("", "lc08-legacy-user-")
	if err != nil {
		t.Fatal(err)
	}
	child.Dir = dir
	child.Env = []string{"APP_HOST=127.0.0.1", fmt.Sprintf("APP_PORT=%d", port), "DB_HOST=127.0.0.1", "DB_PORT=5432", "DB_USER=postgres", "DB_PASSWORD=", "DB_NAME=lc08_producer_qualification_test", "DB_SSLMODE=disable", "DB_MIGRATE_ON_START=false", "NATS_URL=nats://127.0.0.1:4222", "NATS_SUBJECT_USER_CREATE=lc08.qualification.user.create", "INTERNAL_API_TOKEN=synthetic"}
	if err = child.Start(); err != nil {
		if errors.Is(err, syscall.ENOEXEC) {
			t.Skip("legacy Linux/amd64 cannot execute on isolated platform")
		}
		t.Fatal("legacy start failed")
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	finished := false
	defer func() {
		if finished {
			_ = os.RemoveAll(dir)
		} else {
			t.Logf("UNKNOWN: isolated child PID %d and fixture retained for explicit cleanup", child.Process.Pid)
		}
	}()
	ready := false
	for i := 0; i < 80; i++ {
		select {
		case <-done:
			finished = true
			t.Fatal("legacy process exited before readiness")
		default:
		}
		client := http.Client{Timeout: 100 * time.Millisecond}
		resp, e := client.Get(fmt.Sprintf("http://127.0.0.1:%d/internal/health", port))
		if e == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("legacy readiness timed out")
	}
	if err = child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal("signal isolated legacy child")
	}
	select {
	case err = <-done:
		finished = true
		if err != nil {
			t.Fatalf("legacy exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("legacy idle shutdown timeout")
	}
	t.Log("exact old User process idle startup/stop in isolated namespace only; in-flight, admission and external-effect outcomes unproven")
}
