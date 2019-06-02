package mcrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// IRunner interface to running object
type IRunner interface {
	Start() error
}

// Settings encapsulates some basic settings for the server.
type Settings struct {
	Name       string
	MaxRAM     int
	MaxPlayers int
}

// McRunner encapsulates the idea of running a minecraft server.
type McRunner struct {
	Directory string
	Settings  Settings

	cmd  *exec.Cmd
	sock *websocket.Conn

	inMut  sync.Mutex
	outMut sync.Mutex

	inPipe  io.WriteCloser
	outPipe io.ReadCloser
}

// Start the runner.
func (runner *McRunner) Start() error {
	var err error
	// Listen for the bot to establish a connection with us.
	s := http.Server{Addr: ":8080", Handler: nil}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, innerErr := upgrader.Upgrade(w, r, nil)

		runner.sock = ws
		err = innerErr

		// Close http server.
		s.Shutdown(context.Background())

		// Start websocket listener.
		go runner.listen()

		// Start up server.
		runner.run()
	})
	s.ListenAndServe()
	return err
}

// Run the minecraft server and start background listeners.
func (runner *McRunner) run() error {
	// Start the minecraft server.
	runner.applySettings()
	runner.cmd = exec.Command("java", "-jar forge-*.jar", "-Xms512M", fmt.Sprintf("-Xmx%dM", runner.Settings.MaxRAM), "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA")
	runner.inPipe, _ = runner.cmd.StdinPipe()
	runner.outPipe, _ = runner.cmd.StdoutPipe()
	err := runner.cmd.Start()

	go runner.handleOutput()
	go runner.updateStatus()
	go runner.keepAlive()

	return err
}

type header struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type status struct {
	Name        string `json:"name"`
	PlayerCount int    `json:"playercount"`
	PlayerMax   int    `json:"playermax"`
	ActiveTime  int    `json:"activetime"`
	Status      string `json:"status"`
	Memory      int    `json:"memory"`
	MemoryMax   int    `json:"memorymax"`
	Storage     int    `json:"storage"`
	StorageMax  int    `json:"storagemax"`
	//TODO; TPS
	//Data        json.RawMessage `json:"tps"`
}

// Listen for messages from the bot.
func (runner *McRunner) listen() {

}

// Send chat messages to bot.
func (runner *McRunner) handleOutput() {
	for !runner.cmd.ProcessState.Exited() {

	}
}

// Periodically send status updates to bot.
func (runner *McRunner) updateStatus() {
	for !runner.cmd.ProcessState.Exited() {
		status := status{
			Name:        runner.Settings.Name,
			PlayerCount: -1,
			PlayerMax:   runner.Settings.MaxPlayers,
			ActiveTime:  -1,
			Status:      "?",
			Memory:      -1,
			MemoryMax:   runner.Settings.MaxRAM,
			Storage:     -1,
			StorageMax:  -1,
		}
		statusJSON, err := json.Marshal(status)

		if err != nil {
			fmt.Print(err)
		}

		header := header{Type: "status", Data: statusJSON}

		err = runner.sock.WriteJSON(header)

		if err != nil {
			fmt.Print(err)
		}

		time.Sleep(30 * time.Second)
	}
}

// Get the server TPS.
func (runner *McRunner) getTPS() map[int]float32 {
	m := make(map[int]float32)

	runner.inMut.Lock()
	runner.outMut.Lock()
	defer runner.inMut.Unlock()
	defer runner.outMut.Unlock()

	runner.inPipe.Write([]byte("forge tps"))

	//TODO; Finish.
	return m
}

// Monitor the server to restart it when it goes down.
func (runner *McRunner) keepAlive() {
	for !runner.cmd.ProcessState.Exited() {
		time.Sleep(60 * time.Second)
	}
	runner.run()
}

// Update server settings.
func (runner *McRunner) applySettings() {
	//TODO; Replace stuff in the properties file.
}
