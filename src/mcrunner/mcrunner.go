package mcrunner

// IRunner interface to running object
type IRunner interface {
	Start() error
}

// McRunner encapsulates the idea of running a minecraft server.
type McRunner struct {
	Directory string
}

func (runner *McRunner) Start() error {
	return nil
}
