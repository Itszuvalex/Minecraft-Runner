package mcrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
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
	MOTD       string
	MaxRAM     int
	MaxPlayers int
	Port       int
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

	inPipe  io.WriteCloser
	inMutex sync.Mutex
	outPipe io.ReadCloser
	cmd     *exec.Cmd

	firstStart bool

	playerCount int
	tps         map[int]float32

	killChannel   chan bool
	tpsChannel    chan map[int]float32
	playerChannel chan int
}

// Start initializes the runner and starts the minecraft server up.
func (runner *McRunner) Start() error {
	runner.applySettings()
	runner.cmd = exec.Command("java", "-jar forge-*.jar", "-Xms512M", fmt.Sprintf("-Xmx%dM", runner.Settings.MaxRAM), "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA")
	runner.inPipe, _ = runner.cmd.StdinPipe()
	runner.outPipe, _ = runner.cmd.StdoutPipe()
	err := runner.cmd.Start()

	if runner.firstStart {
		runner.firstStart = false

		go runner.keepAlive()
		go runner.updateStatus()
		go runner.processCommands()
	}

	return err
}

// applySettings applies the Settings struct contained in McRunner.
func (runner *McRunner) applySettings() {

}

// processOutput monitors and processes output from the server.
func (runner *McRunner) processOutput() {
	for {
		buf := make([]byte, 256)
		n, err := runner.outPipe.Read(buf)
		str := string(buf[:n])

		if (err == nil) && (n > 1) {
			msgExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: <.*>")
			tpsExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: Dim")
			playerExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: There are")

			if msgExp.Match(buf) {
				runner.MessageChannel <- str[strings.Index(str, ":")+1:]
			} else if tpsExp.Match(buf) {
				content := str[strings.Index(str, "Dim"):]

				numExp, _ := regexp.Compile("[+-]?([0-9]*[.])?[0-9]+")
				nums := numExp.FindAllString(content, -1)
				dim, _ := strconv.Atoi(nums[0])
				tps, _ := strconv.ParseFloat(nums[len(nums)-1], 32)

				m := make(map[int]float32)
				m[dim] = float32(tps)

				runner.tpsChannel <- m
			} else if playerExp.Match(buf) {
				content := str[strings.Index(str, "there"):]

				numExp, _ := regexp.Compile("[+-]?([0-9]*[.])?[0-9]+")
				players, _ := strconv.Atoi(numExp.FindString(content))

				runner.playerChannel <- players
			}
		}
	}
}

// keepAlive monitors the minecraft server and restarts it if it dies.
func (runner *McRunner) keepAlive() {
	for {
		runner.updateState()

		if runner.State == NotRunning {
			runner.Start()
		}

		time.Sleep(5 * time.Second)
	}
}

// updateState updates the state of the server.
func (runner *McRunner) updateState() {
	if (runner.State != NotRunning) && (runner.cmd.ProcessState.Exited()) {
		runner.State = NotRunning
	}
}

// updateStatus sends the status to the BotHandler when requested.
func (runner *McRunner) updateStatus() {

}

// processCommands processes commands from the discord bot.
func (runner *McRunner) processCommands() {

}

// executeCommand is a helper function to execute commands.
func (runner *McRunner) executeCommand(command string) {
	runner.inMutex.Lock()
	defer runner.inMutex.Unlock()

	runner.inPipe.Write([]byte(command))
}
