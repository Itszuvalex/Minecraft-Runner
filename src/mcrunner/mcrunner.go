package mcrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"
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
	Directory         string
	Name              string
	MOTD              string
	ListenAddress     string
	MaxRAM            int
	MaxPlayers        int
	Port              int
	PassthroughStdErr bool
	PassthroughStdOut bool
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
	Storage     uint64          `json:"storage"`
	StorageMax  uint64          `json:"storagemax"`
	TPS         json.RawMessage `json:"tps"`
}

// McRunner encapsulates the idea of running a minecraft server.
type McRunner struct {
	FirstStart bool
	Settings   Settings
	State      State

	WaitGroup            sync.WaitGroup
	StatusRequestChannel chan bool
	StatusChannel        chan *Status
	MessageChannel       chan string
	CommandChannel       chan string

	inPipe    io.WriteCloser
	inMutex   sync.Mutex
	outPipe   io.ReadCloser
	cmd       *exec.Cmd
	startTime time.Time

	killChannel   chan bool
	tpsChannel    chan map[int]float32
	playerChannel chan int
}

// Start initializes the runner and starts the minecraft server up.
func (runner *McRunner) Start() error {
	if runner.State != NotRunning {
		return nil
	}

	runner.applySettings()
	runner.cmd = exec.Command("java", "-jar", "forge-universal.jar", "-Xms512M", fmt.Sprintf("-Xmx%dM", runner.Settings.MaxRAM), "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA", "nogui")
	runner.inPipe, _ = runner.cmd.StdinPipe()
	runner.outPipe, _ = runner.cmd.StdoutPipe()
	if runner.Settings.PassthroughStdErr {
		runner.cmd.Stderr = os.Stderr
	}
	err := runner.cmd.Start()
	if err != nil {
		fmt.Print(err)
		return err
	}
	runner.State = Starting
	runner.startTime = time.Now()

	if runner.FirstStart {
		runner.FirstStart = false

		// Initialize McRunner members that aren't initialized yet.
		runner.killChannel = make(chan bool, 3)
		runner.tpsChannel = make(chan map[int]float32, 8)
		runner.playerChannel = make(chan int, 1)

		go runner.keepAlive()
		go runner.updateStatus()
		go runner.processCommands()
		go runner.processOutput()
	}

	return err
}

// applySettings applies the Settings struct contained in McRunner.
func (runner *McRunner) applySettings() {
	var propPathBuilder strings.Builder
	propPathBuilder.WriteString(runner.Settings.Directory)
	propPathBuilder.WriteString("server.properties")
	propPath := propPathBuilder.String()
	props, err := ioutil.ReadFile(propPath)

	if err != nil {
		fmt.Println(err)
		return
	}

	nameExp, _ := regexp.Compile("displayname=.*\\n")
	motdExp, _ := regexp.Compile("motd=.*\\n")
	maxPlayersExp, _ := regexp.Compile("max-players=.*\\n")
	portExp, _ := regexp.Compile("server-port=.*\\n")

	name := fmt.Sprintf("displayname=%s\n", runner.Settings.Name)
	motd := fmt.Sprintf("motd=%s\n", runner.Settings.MOTD)
	maxPlayers := fmt.Sprintf("max-players=%d\n", runner.Settings.MaxPlayers)
	port := fmt.Sprintf("server-port=%d\n", runner.Settings.Port)

	newProps := strings.Replace(string(props), nameExp.FindString(string(props)), name, 1)
	newProps = strings.Replace(newProps, motdExp.FindString(newProps), motd, 1)
	newProps = strings.Replace(newProps, maxPlayersExp.FindString(newProps), maxPlayers, 1)
	newProps = strings.Replace(newProps, portExp.FindString(newProps), port, 1)

	err = ioutil.WriteFile(propPath, []byte(newProps), 0644)

	if err != nil {
		fmt.Println(err)
		return
	}
}

// processOutput monitors and processes output from the server.
func (runner *McRunner) processOutput() {
	runner.WaitGroup.Add(1)
	defer runner.WaitGroup.Done()
	for {
		select {
		case <-runner.killChannel:
			return
		default:
			buf := make([]byte, 256)
			n, err := runner.outPipe.Read(buf)
			str := string(buf[:n])

			if (err == nil) && (n > 1) {
				if runner.Settings.PassthroughStdOut {
					fmt.Print(str)
				}
				msgExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: <.*>")
				tpsExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: Dim")
				playerExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: There are")
				doneExp, _ := regexp.Compile("\\[.*\\] \\[.*INFO\\] \\[.*DedicatedServer\\]: Done")

				if runner.State == Starting {
					if doneExp.Match(buf) {
						runner.State = Running
						fmt.Println("Minecraft server done loading.")
					}
				} else if runner.State == Running {
					if msgExp.Match(buf) {
						runner.MessageChannel <- str[strings.Index(str, "<"):]
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
						content := str[strings.Index(str, "There"):]

						numExp, _ := regexp.Compile("[+-]?([0-9]*[.])?[0-9]+")
						players, _ := strconv.Atoi(numExp.FindString(content))

						runner.playerChannel <- players
					}
				}
			}
		}
	}
}

// keepAlive monitors the minecraft server and restarts it if it dies.
func (runner *McRunner) keepAlive() {
	runner.WaitGroup.Add(1)
	defer runner.WaitGroup.Done()
	for {
		select {
		default:
			state, err := runner.cmd.Process.Wait()
			if state.Exited() {
				runner.State = NotRunning
				if err != nil {
					fmt.Println(err)
				}
				runner.Start()
			}
		case <-runner.killChannel:
			return
		}
	}
}

// updateStatus sends the status to the BotHandler when requested.
func (runner *McRunner) updateStatus() {
	runner.WaitGroup.Add(1)
	defer runner.WaitGroup.Done()
	for {
		select {
		case <-runner.StatusRequestChannel:
			if runner.State != Running {
				continue
			}

			status := new(Status)
			status.Name = runner.Settings.Name
			status.PlayerMax = runner.Settings.MaxPlayers
			switch runner.State {
			case NotRunning:
				status.Status = "Not Running"
			case Starting:
				status.Status = "Starting"
			case Running:
				status.Status = "Running"
			}
			status.ActiveTime = int(time.Since(runner.startTime).Seconds())

			proc, _ := process.NewProcess(int32(runner.cmd.Process.Pid))
			memInfo, _ := proc.MemoryInfo()
			status.MemoryMax = runner.Settings.MaxRAM
			status.Memory = int(memInfo.RSS / (1024 * 1024))

			var worldPathBuilder strings.Builder
			worldPathBuilder.WriteString(runner.Settings.Directory)
			worldPathBuilder.WriteString("world/")
			worldPath := worldPathBuilder.String()
			usage, _ := disk.Usage(worldPath)
			status.Storage = usage.Used / (1024 * 1024)
			status.StorageMax = usage.Total / (1024 * 1024)

			runner.executeCommand("list")
			status.PlayerCount = <-runner.playerChannel

			tpsMap := make(map[int]float32)
			runner.executeCommand("forge tps")
		loop:
			for {
				select {
				case m := <-runner.tpsChannel:
					for k, v := range m {
						tpsMap[k] = v
					}
				case <-time.After(1 * time.Second):
					break loop
				}
			}
			var tpsStrBuilder strings.Builder
			tpsStrBuilder.WriteString("{ ")
			for k, v := range tpsMap {
				tpsStrBuilder.WriteString(fmt.Sprintf("\"%d\": %f, ", k, v))
			}
			tpsStr := tpsStrBuilder.String()[:tpsStrBuilder.Len()-3]
			tpsStrBuilder.Reset()
			tpsStrBuilder.WriteString(tpsStr)
			tpsStrBuilder.WriteString("}")
			tpsStr = tpsStrBuilder.String()
			status.TPS = []byte(tpsStr)

			runner.StatusChannel <- status
		case <-runner.killChannel:
			return
		}
	}
}

// processCommands processes commands from the discord bot.
func (runner *McRunner) processCommands() {
	runner.WaitGroup.Add(1)
	defer runner.WaitGroup.Done()
	for {
		select {
		case command := <-runner.CommandChannel:
			switch command {
			case "start":
				if runner.State == NotRunning {
					runner.Start()
				}
			case "stop":
				runner.executeCommand("stop")
				runner.killChannel <- true
				runner.killChannel <- true
				runner.killChannel <- true
				runner.State = NotRunning
			case "kill":
				runner.cmd.Process.Kill()
				runner.killChannel <- true
				runner.killChannel <- true
				runner.killChannel <- true
				runner.State = NotRunning
			case "reboot":
				runner.executeCommand("stop")
				time.Sleep(5 * time.Second)
				runner.Start()
			case "forcereboot":
				runner.cmd.Process.Kill()
				time.Sleep(5 * time.Second)
				runner.Start()
			case "save":
				runner.executeCommand("save-all")
			default:
				runner.executeCommand(command)
			}
			runner.executeCommand(command)
		case <-runner.killChannel:
			return
		}

	}
}

// executeCommand is a helper function to execute commands.
func (runner *McRunner) executeCommand(command string) {
	runner.inMutex.Lock()
	defer runner.inMutex.Unlock()

	runner.inPipe.Write([]byte(command))
	runner.inPipe.Write([]byte("\n"))
}
