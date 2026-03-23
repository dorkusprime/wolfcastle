package validate

// PIDChecker abstracts the daemon lifecycle queries that the validation
// engine needs. The concrete implementation lives in the daemon package;
// accepting this interface here keeps validate free of that dependency.
type PIDChecker interface {
	IsAlive() bool
	PIDFileExists() bool
	StopFileExists() bool
	RemovePID() error
	RemoveStopFile() error
}
