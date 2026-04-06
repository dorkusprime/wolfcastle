package validate

// PIDChecker abstracts the daemon lifecycle queries that the validation
// engine needs. The concrete implementation lives in the daemon package;
// accepting this interface here keeps validate free of that dependency.
//
// IsAlive checks the instance registry (not a PID file) for a live daemon.
// StopFileExists and RemoveStopFile operate on the stop sentinel file.
type PIDChecker interface {
	IsAlive() bool
	StopFileExists() bool
	RemoveStopFile() error
}
