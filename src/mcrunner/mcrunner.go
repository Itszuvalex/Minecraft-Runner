package mcrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
)

// IRunner interface to running object
type IRunner interface {
	Start() error
}

// McRunner encapsulates the idea of running a minecraft server.
type McRunner struct {
	Directory string
	Cmd       *exec.Cmd
	Sock      *websocket.Conn
}

// Start the runner.
func (runner *McRunner) Start() error {
	var err error
	// Listen for the bot to establish a connection with us.
	s := http.Server{Addr: ":8080", Handler: nil}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, innerErr := upgrader.Upgrade(w, r, nil)

		runner.Sock = ws
		err = innerErr

		// Close http server.
		s.Shutdown(context.Background())
	})
	s.ListenAndServe()
	return err
}

// Run the minecraft server and start background listeners.
func (runner *McRunner) run() error {
	// Start the minecraft server.
	runner.Cmd = exec.Command("java", "-jar forge-*.jar", "-Xms1G", "-Xmx6G", "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA")
	err := runner.Cmd.Start()

	go runner.listen()
	go runner.handleOutput()
	go runner.updateStatus()

	return err
}

type Header struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Status struct {
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

// Listen for commands from bot.
func (runner *McRunner) listen() {

}

// Send chat messages to bot.
func (runner *McRunner) handleOutput() {

}

// Periodically send status updates to bot.
func (runner *McRunner) updateStatus() {
	status := Status{
		Name:        "?",
		PlayerCount: -1,
		PlayerMax:   -1,
		ActiveTime:  -1,
		Status:      "?",
		Memory:      -1,
		MemoryMax:   6192,
		Storage:     -1,
		StorageMax:  -1,
	}
	statusJSON, err := json.Marshal(status)

	if err != nil {
		fmt.Print(err)
	}

	header := Header{Type: "status", Data: statusJSON}

	err = runner.Sock.WriteJSON(header)

	if err != nil {
		fmt.Print(err)
	}

	time.Sleep(60 * time.Second)
}
