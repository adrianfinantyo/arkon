package cli

// Standard deterministic exit codes for Arkon.
const (
	// ExitOK indicates successful execution.
	ExitOK = 0

	// ExitGeneralError indicates an unexpected runtime failure or argument error.
	ExitGeneralError = 1

	// ExitInhibited indicates power transition was blocked by an active lock or daytime protection.
	ExitInhibited = 2

	// ExitValidationFailure indicates manifest syntax or semantic validation failed.
	ExitValidationFailure = 3

	// ExitInsufficientPrivilege indicates elevated root access is required.
	ExitInsufficientPrivilege = 4
)
