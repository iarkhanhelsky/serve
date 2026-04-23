package types

// RunOptions is the normalized runtime configuration for the serve command.
type RunOptions struct {
	Root       string
	Listen     string
	Upstream   string
	LogMode    string
	ErrorsOnly bool
	LogFile    string
}
