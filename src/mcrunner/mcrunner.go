package mcrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// IRunner interface to running object
type IRunner interface {
	Start() error
}

// State exists because Go doesn't have enums for some reason.
type State int

const (
	// NotRunning indicates something...
	NotRunning State = 0
	// Starting indicates the server is running but not ready for players yet.
	Starting State = 1
	// Running indicates the server is ready for players to connect to.
	Running State = 2
)

// Settings encapsulates some basic settings for the server.
type Settings struct {
	Name       string
	MaxRAM     int
	MaxPlayers int
}

// Status stores information on the status of the minecraft server.
type Status struct {
	Name        string          `json:"name"`
	PlayerCount int             `json:"playercount"`
	PlayerMax   int             `json:"playermax"`
	ActiveTime  int             `json:"activetime"`
	Status      string          `json:"status"`
	Memory      int             `json:"memory"`
	MemoryMax   int             `json:"memorymax"`
	Storage     int             `json:"storage"`
	StorageMax  int             `json:"storagemax"`
	TPS         json.RawMessage `json:"tps"`
}

// McRunner encapsulates the idea of running a minecraft server.
type McRunner struct {
	Directory string
	Settings  Settings
	State     State

	StatusRequestChannel chan bool
	StatusChannel        chan Status
	MessageChannel       chan string
	CommandChannel       chan string

	inPipe     io.WriteCloser
	outPipe    io.ReadCloser
	cmd        *exec.Cmd
	firstStart bool

	killChannel chan bool
}

// Start initializes the runner and starts the minecraft server up.
func (runner *McRunner) Start() error {
	runner.applySettings()
	runner.cmd = exec.Command("java", "-jar forge-*.jar", "-Xms512M", fmt.Sprintf("-Xmx%dM", runner.Settings.MaxRAM), "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA")
	runner.inPipe, _ = runner.cmd.StdinPipe()
	runner.outPipe, _ = runner.cmd.StdoutPipe()
	err := runner.cmd.Start()

	return err
}

// applySettings applies the Settings struct contained in McRunner.
func (runner *McRunner) applySettings() {

}

// keepAlive monitors the minecraft server and restarts it if it dies.
func (runner *McRunner) keepAlive() {

}

// updateStatus sends the status to the BotHandler when requested.
func (runner *McRunner) updateStatus() {

}

// processCommands processes commands from the discord bot.
func (runner *McRunner) processCommands() {

}

// executeCommand is a helper function to execute commands.
func (runner *McRunner) executeCommand(command string) {

}
