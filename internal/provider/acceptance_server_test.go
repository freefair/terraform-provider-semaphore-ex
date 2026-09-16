package provider

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") == "" || os.Getenv("SEMAPHORE_EX_TEST_BINARY") == "" {
		os.Exit(m.Run())
	}
	fixture, err := startAcceptanceServer()
	if err != nil {
		fmt.Fprintln(os.Stderr, "start isolated Semaphore EX acceptance server:", err)
		os.Exit(1)
	}
	code := m.Run()
	fixture.close(code != 0)
	os.Exit(code)
}

type acceptanceServer struct {
	dir     string
	command *exec.Cmd
	log     *os.File
}

func startAcceptanceServer() (*acceptanceServer, error) {
	dir, err := os.MkdirTemp("", "semaphore-ex-provider-acceptance-")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*acceptanceServer, error) {
		return nil, fmt.Errorf("%w (fixture directory retained at %s)", err, dir)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fail(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	key := func() (string, error) {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		return base64.StdEncoding.EncodeToString(b), err
	}
	hash, err := key()
	if err != nil {
		return fail(err)
	}
	cipher, err := key()
	if err != nil {
		return fail(err)
	}
	_, port, _ := net.SplitHostPort(address)
	config := fmt.Sprintf("dialect: sqlite\nsqlite:\n  host: %q\ninterface: 127.0.0.1\nport: %q\ntmp_path: %q\n", filepath.Join(dir, "semaphore.db"), ":"+port, filepath.Join(dir, "tmp"))
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fail(err)
	}
	env := append(isolatedAcceptanceEnv(), "SEMAPHORE_COOKIE_HASH="+hash, "SEMAPHORE_COOKIE_ENCRYPTION="+cipher, "SEMAPHORE_ACCESS_KEY_ENCRYPTION="+cipher)
	binary := os.Getenv("SEMAPHORE_EX_TEST_BINARY")
	setupLog, err := os.OpenFile(filepath.Join(dir, "setup.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fail(err)
	}
	run := func(args ...string) ([]byte, error) {
		c := exec.Command(binary, append([]string{"--config", configPath}, args...)...)
		c.Env = env
		var stderr bytes.Buffer
		c.Stderr = &stderr
		out, err := c.Output()
		if err != nil {
			_, _ = setupLog.Write(append(stderr.Bytes(), '\n'))
		}
		return out, err
	}
	if _, err := run("migrate"); err != nil {
		return fail(fmt.Errorf("migrate: %w", err))
	}
	if _, err := run("user", "add", "--login", "acceptance-admin", "--name", "Acceptance Admin", "--email", "acceptance-admin@example.test", "--external", "--admin"); err != nil {
		return fail(fmt.Errorf("create synthetic admin: %w", err))
	}
	tokenOutput, err := run("user", "token", "create", "--login", "acceptance-admin", "--name", "provider-acceptance", "--ttl", "1h")
	token := lastNonEmptyLine(string(tokenOutput))
	if err != nil || !validAcceptanceToken(token) {
		return fail(fmt.Errorf("create synthetic API token failed"))
	}
	log, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fail(err)
	}
	cmd := exec.Command(binary, "--config", configPath, "server")
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return fail(err)
	}
	baseURL := "http://" + address + "/api"
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/ping")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				_ = os.Setenv("SEMAPHOREUI_API_BASE_URL", baseURL)
				_ = os.Setenv("SEMAPHOREUI_API_TOKEN", strings.TrimSpace(string(token)))
				_ = setupLog.Close()
				return &acceptanceServer{dir, cmd, log}, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = log.Close()
	return fail(fmt.Errorf("server did not become ready; inspect %s", filepath.Join(dir, "server.log")))
}

func isolatedAcceptanceEnv() []string {
	var env []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "SEMAPHORE_") {
			env = append(env, value)
		}
	}
	return env
}

func lastNonEmptyLine(value string) string {
	for _, line := range strings.Fields(value) {
		value = line
	}
	return value
}
func validAcceptanceToken(value string) bool {
	_, err := base64.URLEncoding.DecodeString(value)
	return err == nil && value != ""
}

func (s *acceptanceServer) close(retain bool) {
	if s.command.Process != nil {
		_ = s.command.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = s.command.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = s.command.Process.Kill()
			<-done
		}
	}
	_ = s.log.Close()
	if !retain {
		_ = os.RemoveAll(s.dir)
	}
}
