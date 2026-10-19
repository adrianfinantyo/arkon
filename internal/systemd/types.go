package systemd

import "errors"

// Default unit and path constants.
const (
	DefaultServiceName = "arkon-sleep.service"
	DefaultTimerName   = "arkon-sleep.timer"
	DefaultTargetDir   = "/etc/systemd/system"
)

// Standard systemd installation errors.
var (
	ErrInsufficientPrivileges = errors.New("insufficient privileges: root access (EUID == 0) required")
	ErrInvalidManifest       = errors.New("cannot compile units from nil manifest")
	ErrVerificationFailed    = errors.New("systemd unit verification failed")
)

// CompileOptions defines inputs for unit template compilation.
type CompileOptions struct {
	BinaryPath string
	ConfigPath string
}

// CompiledUnits holds compiled systemd unit file contents and names.
type CompiledUnits struct {
	ServiceName    string
	TimerName      string
	ServicePath    string
	TimerPath      string
	ServiceContent string
	TimerContent   string
}

// InstallOptions defines configuration parameters for atomic unit installation.
type InstallOptions struct {
	DryRun     bool
	TargetDir  string
	StagingDir string
	BinaryPath string
	ConfigPath string
}

// ApplyData represents the structured result returned by arkon apply.
type ApplyData struct {
	Manifest      string            `json:"manifest"`
	DryRun        bool              `json:"dry_run"`
	Validated     bool              `json:"validated"`
	Units         []string          `json:"units"`
	UnitContents  map[string]string `json:"unit_contents,omitempty"`
	SystemdReload bool              `json:"systemd_reload"`
	TimerEnabled  bool              `json:"timer_enabled"`
	NextTrigger   string            `json:"next_trigger,omitempty"`
}
