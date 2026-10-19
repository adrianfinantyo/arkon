package systemd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/state"
)

func sampleManifest() *manifest.Manifest {
	m := &manifest.Manifest{
		Version: "1",
		Host: manifest.HostConfig{
			Timezone:    "Asia/Jakarta",
			RTCDevice:   "/sys/class/rtc/rtc0",
			PowerAction: "poweroff",
		},
		Circadian: manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		},
		Teardown: manifest.TeardownConfig{
			Provider:       "docker-compose",
			TimeoutSeconds: 30,
			ProfileOrder:   []string{"apps", "infra"},
		},
	}
	m.ApplyDefaults()
	return m
}

func TestCompileNilManifest(t *testing.T) {
	_, err := Compile(nil, CompileOptions{})
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("expected ErrInvalidManifest, got %v", err)
	}
}

func TestCompileValidManifest(t *testing.T) {
	m := sampleManifest()
	opts := CompileOptions{
		BinaryPath: "/usr/local/bin/arkon",
		ConfigPath: "/etc/arkon/arkon.yaml",
	}

	compiled, err := Compile(m, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if compiled.ServiceName != DefaultServiceName {
		t.Errorf("expected service name %s, got %s", DefaultServiceName, compiled.ServiceName)
	}
	if compiled.TimerName != DefaultTimerName {
		t.Errorf("expected timer name %s, got %s", DefaultTimerName, compiled.TimerName)
	}

	// Verify Service content
	if !strings.Contains(compiled.ServiceContent, "Type=oneshot") {
		t.Errorf("expected Type=oneshot in service unit")
	}
	if !strings.Contains(compiled.ServiceContent, "ExecStart=/usr/local/bin/arkon sleep --config /etc/arkon/arkon.yaml") {
		t.Errorf("expected ExecStart in service unit, got:\n%s", compiled.ServiceContent)
	}
	if !strings.Contains(compiled.ServiceContent, "WantedBy=multi-user.target") {
		t.Errorf("expected WantedBy=multi-user.target in service unit")
	}

	// Verify Timer content
	if !strings.Contains(compiled.TimerContent, "Persistent=false") {
		t.Errorf("expected Persistent=false in timer unit")
	}
	if !strings.Contains(compiled.TimerContent, "OnCalendar=Mon..Sun *-*-* 23:00:00") {
		t.Errorf("expected OnCalendar=Mon..Sun *-*-* 23:00:00, got:\n%s", compiled.TimerContent)
	}
	if !strings.Contains(compiled.TimerContent, "Unit=arkon-sleep.service") {
		t.Errorf("expected Unit=arkon-sleep.service in timer unit")
	}
	if !strings.Contains(compiled.TimerContent, "WantedBy=timers.target") {
		t.Errorf("expected WantedBy=timers.target in timer unit")
	}

	// Verify Boot service content
	if !compiled.BootEnabled {
		t.Errorf("expected BootEnabled = true")
	}
	if !strings.Contains(compiled.BootServiceContent, "Type=oneshot") {
		t.Errorf("expected Type=oneshot in boot service unit")
	}
	if !strings.Contains(compiled.BootServiceContent, "ExecStart=/usr/local/bin/arkon up --config /etc/arkon/arkon.yaml") {
		t.Errorf("expected ExecStart in boot service unit, got:\n%s", compiled.BootServiceContent)
	}
	if !strings.Contains(compiled.BootServiceContent, "WantedBy=multi-user.target") {
		t.Errorf("expected WantedBy=multi-user.target in boot service unit")
	}
}

func TestFormatOnCalendar(t *testing.T) {
	tests := []struct {
		name     string
		days     []string
		sleepAt  string
		expected string
	}{
		{
			name:     "seven days full week",
			days:     []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
			sleepAt:  "23:00",
			expected: "Mon..Sun *-*-* 23:00:00",
		},
		{
			name:     "five weekdays",
			days:     []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
			sleepAt:  "22:30",
			expected: "Mon,Tue,Wed,Thu,Fri *-*-* 22:30:00",
		},
		{
			name:     "empty days defaults to Mon..Sun",
			days:     []string{},
			sleepAt:  "23:00",
			expected: "Mon..Sun *-*-* 23:00:00",
		},
		{
			name:     "single weekend day",
			days:     []string{"Sun"},
			sleepAt:  "01:15",
			expected: "Sun *-*-* 01:15:00",
		},
		{
			name:     "sleepAt with seconds already specified",
			days:     []string{"Mon"},
			sleepAt:  "23:00:45",
			expected: "Mon *-*-* 23:00:45",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatOnCalendar(tc.days, tc.sleepAt)
			if got != tc.expected {
				t.Errorf("FormatOnCalendar() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestValidateUnitSyntax(t *testing.T) {
	validService := "[Unit]\nDescription=Test\n[Service]\nType=oneshot\nExecStart=/bin/true\n"
	validTimer := "[Unit]\nDescription=Test\n[Timer]\nOnCalendar=*-*-* 00:00:00\nPersistent=false\n[Install]\nWantedBy=timers.target\n"

	if err := ValidateUnitSyntax(validService, validTimer); err != nil {
		t.Fatalf("expected valid syntax, got: %v", err)
	}

	// Missing [Service]
	if err := ValidateUnitSyntax("[Unit]\nType=oneshot\nExecStart=/bin/true", validTimer); err == nil {
		t.Errorf("expected error for missing [Service]")
	}

	// Missing Type=oneshot
	if err := ValidateUnitSyntax("[Unit]\n[Service]\nExecStart=/bin/true", validTimer); err == nil {
		t.Errorf("expected error for missing Type=oneshot")
	}

	// Missing Persistent=false
	if err := ValidateUnitSyntax(validService, "[Unit]\n[Timer]\nOnCalendar=*-*-*\n[Install]\nWantedBy=timers.target"); err == nil {
		t.Errorf("expected error for missing Persistent=false")
	}

	// Missing [Install]
	if err := ValidateUnitSyntax(validService, "[Unit]\n[Timer]\nOnCalendar=*-*-*\nPersistent=false"); err == nil {
		t.Errorf("expected error for missing [Install]")
	}
}

func TestInstallerRootEnforcement(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000 // Non-root
	installer := NewInstaller(mem)
	m := sampleManifest()

	// Normal run without dry-run must fail with ErrInsufficientPrivileges
	_, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:    false,
		TargetDir: "/etc/systemd/system",
	})
	if !errors.Is(err, ErrInsufficientPrivileges) {
		t.Fatalf("expected ErrInsufficientPrivileges, got: %v", err)
	}

	// With dry-run, non-root user must succeed
	data, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     true,
		TargetDir:  "/etc/systemd/system",
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err != nil {
		t.Fatalf("expected dry-run to succeed for non-root, got: %v", err)
	}
	if !data.DryRun {
		t.Errorf("expected ApplyData.DryRun = true")
	}
	if !data.Validated {
		t.Errorf("expected ApplyData.Validated = true")
	}
	if len(mem.Files) != 0 {
		t.Errorf("expected 0 files written to disk in dry-run, found: %v", mem.Files)
	}
}

func TestInstallerDryRunOutput(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0
	installer := NewInstaller(mem)
	m := sampleManifest()

	data, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     true,
		TargetDir:  "/etc/systemd/system",
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	if data.SystemdReload {
		t.Errorf("expected SystemdReload=false in dry-run")
	}
	if data.TimerEnabled {
		t.Errorf("expected TimerEnabled=false in dry-run")
	}
	if data.UnitContents == nil || data.UnitContents[DefaultServiceName] == "" {
		t.Errorf("expected UnitContents for service in dry-run")
	}
	if data.UnitContents[DefaultTimerName] == "" {
		t.Errorf("expected UnitContents for timer in dry-run")
	}
}

func TestInstallerAtomicInstallSuccess(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	var commandsExecuted []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		commandsExecuted = append(commandsExecuted, cmd)
		return []byte("ok"), nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	targetDir := "/etc/systemd/system"
	data, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  targetDir,
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if data.DryRun {
		t.Errorf("expected DryRun = false")
	}
	if !data.SystemdReload {
		t.Errorf("expected SystemdReload = true")
	}
	if !data.TimerEnabled {
		t.Errorf("expected TimerEnabled = true")
	}

	// Verify unit files exist in destination
	svcPath := filepath.Join(targetDir, DefaultServiceName)
	tmrPath := filepath.Join(targetDir, DefaultTimerName)
	bootPath := filepath.Join(targetDir, DefaultBootServiceName)

	if _, exists := mem.Files[svcPath]; !exists {
		t.Fatalf("expected service unit at %s", svcPath)
	}
	if _, exists := mem.Files[tmrPath]; !exists {
		t.Fatalf("expected timer unit at %s", tmrPath)
	}
	if _, exists := mem.Files[bootPath]; !exists {
		t.Fatalf("expected boot unit at %s", bootPath)
	}

	// Verify commands
	if len(commandsExecuted) != 4 {
		t.Fatalf("expected 4 commands, got %v", commandsExecuted)
	}
	if !strings.HasPrefix(commandsExecuted[0], "systemd-analyze verify ") {
		t.Errorf("command 0: expected systemd-analyze verify, got %q", commandsExecuted[0])
	}
	if commandsExecuted[1] != "systemctl daemon-reload" {
		t.Errorf("command 1: expected systemctl daemon-reload, got %q", commandsExecuted[1])
	}
	if commandsExecuted[2] != "systemctl enable --now arkon-sleep.timer" {
		t.Errorf("command 2: expected systemctl enable --now arkon-sleep.timer, got %q", commandsExecuted[2])
	}
	if commandsExecuted[3] != "systemctl enable arkon-boot.service" {
		t.Errorf("command 3: expected systemctl enable arkon-boot.service, got %q", commandsExecuted[3])
	}
}

func TestInstallerRollbackOnFileWriteFailure(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	targetDir := "/etc/systemd/system"
	svcPath := filepath.Join(targetDir, DefaultServiceName)
	tmrPath := filepath.Join(targetDir, DefaultTimerName)

	// Pre-existing units before install
	originalService := []byte("# old service")
	originalTimer := []byte("# old timer")
	mem.Files[svcPath] = originalService
	mem.Files[tmrPath] = originalTimer

	// Fail write on timer unit
	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasPrefix(path, tmrPath) {
			return errors.New("simulated disk full")
		}
		mem.Files[path] = data
		return nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	_, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  targetDir,
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err == nil {
		t.Fatal("expected Install to fail on write error")
	}

	// Verify rollback restored original service
	if string(mem.Files[svcPath]) != string(originalService) {
		t.Errorf("expected service unit to be restored to %q, got %q", string(originalService), string(mem.Files[svcPath]))
	}
	if string(mem.Files[tmrPath]) != string(originalTimer) {
		t.Errorf("expected timer unit to be restored to %q, got %q", string(originalTimer), string(mem.Files[tmrPath]))
	}
}

func TestInstallerRollbackOnRenameFailure(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	targetDir := "/etc/systemd/system"
	svcPath := filepath.Join(targetDir, DefaultServiceName)
	tmrPath := filepath.Join(targetDir, DefaultTimerName)

	originalService := []byte("# old service")
	originalTimer := []byte("# old timer")
	mem.Files[svcPath] = originalService
	mem.Files[tmrPath] = originalTimer

	// Fail rename on timer unit
	mem.RenameHandler = func(oldPath, newPath string) error {
		if newPath == tmrPath {
			return errors.New("simulated rename failure")
		}
		data, exists := mem.Files[oldPath]
		if !exists {
			return os.ErrNotExist
		}
		delete(mem.Files, oldPath)
		mem.Files[newPath] = data
		return nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	_, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  targetDir,
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err == nil {
		t.Fatal("expected Install to fail on rename error")
	}

	// Verify rollback restored original service
	if string(mem.Files[svcPath]) != string(originalService) {
		t.Errorf("expected service unit to be restored to %q, got %q", string(originalService), string(mem.Files[svcPath]))
	}
	if string(mem.Files[tmrPath]) != string(originalTimer) {
		t.Errorf("expected timer unit to be restored to %q, got %q", string(originalTimer), string(mem.Files[tmrPath]))
	}
}

func TestInstallerRollbackOnDaemonReloadFailure(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	targetDir := "/etc/systemd/system"
	svcPath := filepath.Join(targetDir, DefaultServiceName)
	tmrPath := filepath.Join(targetDir, DefaultTimerName)

	reloadCalls := 0
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "daemon-reload" {
			reloadCalls++
			if reloadCalls == 1 {
				return []byte("daemon-reload failed"), errors.New("systemd connection error")
			}
			return []byte("recovery ok"), nil
		}
		return []byte("ok"), nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	_, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  targetDir,
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err == nil {
		t.Fatal("expected Install to fail on daemon-reload error")
	}

	// Since there were no pre-existing units, destination files should have been removed
	if _, exists := mem.Files[svcPath]; exists {
		t.Errorf("expected service unit to be removed after rollback")
	}
	if _, exists := mem.Files[tmrPath]; exists {
		t.Errorf("expected timer unit to be removed after rollback")
	}

	// Recovery reload should have been called
	if reloadCalls < 2 {
		t.Errorf("expected at least 2 daemon-reload calls (failed + rollback), got %d", reloadCalls)
	}
}

func TestInstallerRollbackOnEnableFailure(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	targetDir := "/etc/systemd/system"
	svcPath := filepath.Join(targetDir, DefaultServiceName)
	tmrPath := filepath.Join(targetDir, DefaultTimerName)

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "enable" {
			return []byte("failed to enable unit"), errors.New("unit enablement error")
		}
		return []byte("ok"), nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	_, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  targetDir,
		ConfigPath: "/etc/arkon/arkon.yaml",
	})
	if err == nil {
		t.Fatal("expected Install to fail on enable error")
	}

	// Units should have been rolled back
	if _, exists := mem.Files[svcPath]; exists {
		t.Errorf("expected service unit to be removed after rollback")
	}
	if _, exists := mem.Files[tmrPath]; exists {
		t.Errorf("expected timer unit to be removed after rollback")
	}
}

func TestVerifyStagedUnitsWithSystemdAnalyzeFailure(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0

	svcPath := "/tmp/stage/arkon-sleep.service"
	tmrPath := "/tmp/stage/arkon-sleep.timer"

	mem.Files[svcPath] = []byte("[Unit]\n[Service]\nType=oneshot\nExecStart=/bin/true\n")
	mem.Files[tmrPath] = []byte("[Unit]\n[Timer]\nOnCalendar=*-*-*\nPersistent=false\n[Install]\nWantedBy=timers.target\n")

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemd-analyze" {
			return []byte("bad unit file setting"), errors.New("exit status 1")
		}
		return nil, errors.New("command not mocked")
	}

	err := VerifyStagedUnits(context.Background(), mem, svcPath, tmrPath)
	if err == nil {
		t.Fatal("expected verification to fail when systemd-analyze returns syntax error")
	}
	if !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("expected ErrVerificationFailed, got: %v", err)
	}
}

func TestInstallerActiveManifestRegistration(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0
	stateDir := t.TempDir()

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	installer := NewInstaller(mem)
	m := sampleManifest()

	sourcePath := "/home/adrian/deployment-stack/arkon.yaml"
	mem.Files[sourcePath] = []byte("version: '1'")

	data, err := installer.Install(context.Background(), m, InstallOptions{
		DryRun:     false,
		TargetDir:  "/etc/systemd/system",
		ConfigPath: sourcePath,
		StateDir:   stateDir,
		AppliedBy:  "adrian",
	})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if data.ActiveManifest != sourcePath {
		t.Errorf("expected ActiveManifest %q, got %q", sourcePath, data.ActiveManifest)
	}
	if data.CanonicalLink != "/etc/arkon/arkon.yaml" {
		t.Errorf("expected CanonicalLink /etc/arkon/arkon.yaml, got %q", data.CanonicalLink)
	}

	// Verify symlink was created in mem
	linkTarget, err := mem.Readlink("/etc/arkon/arkon.yaml")
	if err != nil || linkTarget != sourcePath {
		t.Fatalf("expected symlink target %q, got %q (err: %v)", sourcePath, linkTarget, err)
	}

	// Verify state file recorded active manifest
	stateData, err := mem.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatalf("expected state file to exist in mem: %v", err)
	}
	var st state.State
	if err := json.Unmarshal(stateData, &st); err != nil {
		t.Fatalf("failed to parse state json: %v", err)
	}
	if st.ActiveManifest != sourcePath {
		t.Errorf("expected recorded path %q, got %q", sourcePath, st.ActiveManifest)
	}
	if st.AppliedBy != "adrian" {
		t.Errorf("expected appliedBy 'adrian', got %q", st.AppliedBy)
	}
}
