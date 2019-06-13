package mcrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

const (
	// MinecraftServerDirectory name of the directory underneath the main.exe containing all mcserver data
	MinecraftServerDirectory = "mcserver"
	MinecraftServerJar       = "forge-universal.jar"
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

func RootPath() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func McServerPath() string {
	return filepath.Join(RootPath(), MinecraftServerDirectory)
}

func ServerJarName(mcVer string, forgeVer string) string {
	return fmt.Sprintf("forge-%s-%s-universal.jar", mcVer, forgeVer)
}

func (runner *McRunner) Installed() bool {
	_, err := os.Stat(filepath.Join(McServerPath(), MinecraftServerJar))
	return err == nil
}

func DownloadFile(localpath, netpath string, returnIfExists bool) error {
	_, err := os.Stat(localpath)
	if err == nil && returnIfExists {
		return nil
	}

	localpathfile, err := os.Create(localpath)
	if err != nil {
		fmt.Println("Download file: Create file:", err)
		return err
	}
	defer localpathfile.Close()

	response, err := http.Get(netpath)
	if err != nil {
		fmt.Println("DownloadFile: downloading:", err)
		return err
	}
	defer response.Body.Close()

	_, err = io.Copy(localpathfile, response.Body)
	if err != nil {
		fmt.Println("DownloadFile: copying stream:", err)
		return err
	}

	return nil
}

func (runner *McRunner) InstallForgeJar(mcver, forgever string) error {
	installerjarname := "forge-universal.jar"
	installerjarpath := filepath.Join(McServerPath(), installerjarname)
	installernetpath := fmt.Sprintf("https://files.minecraftforge.net/maven/net/minecraftforge/forge/%s-%s/%s", mcver, forgever, ServerJarName(mcver, forgever))
	err := DownloadFile(installerjarpath, installernetpath, true)
	if err != nil {
		fmt.Println("InstallForgeJar:", err)
		return err
	}
	return nil
}

func (runner *McRunner) InstallMinecraftServerJar(mcver string) error {
	jarname := fmt.Sprintf("minecraft_server.%s.jar", mcver)
	netpath := fmt.Sprintf("https://s3.amazonaws.com/Minecraft.Download/versions/%s/%s", mcver, jarname)
	jarpath := filepath.Join(McServerPath(), jarname)
	err := DownloadFile(jarpath, netpath, true)
	if err != nil {
		fmt.Println("InstallMinecraftServerJar: DownloadFile:", err)
		return err
	}
	return nil
}

func (runner *McRunner) InstallLaunchWrapper(wrapperver string) error {
	path := fmt.Sprintf("net/minecraft/launchwrapper/%s/launchwrapper-%s.jar", wrapperver, wrapperver)
	webpath := fmt.Sprintf("https://libraries.minecraft.net/%s", path)
	localpath := filepath.Join(McServerPath(), "libraries", path)
	err := DownloadFile(localpath, webpath, true)
	if err != nil {
		fmt.Println("InstallLaunchWrapper: DownloadFile:", err)
		return err
	}
	return nil
}

func (runner *McRunner) HandleEula() error {
	eulacmd := exec.Command("java", "-jar", "forge-universal.jar", "-Xmx2G", "nogui")
	eulacmd.Dir = McServerPath()
	fmt.Println("Generating eula")
	err := eulacmd.Run()
	if err != nil {
		fmt.Println("HandleEula: Running java:", err)
		return err
	}

	eulafilepath := filepath.Join(McServerPath(), "eula.txt")
	_, err = os.Stat(eulafilepath)
	if err != nil {
		fmt.Println("HandleEula: Stat:", err)
		return err
	}

	eulafile, err := ioutil.ReadFile(eulafilepath)
	if err != nil {
		fmt.Println("HandleEula: ReadFile:", err)
		return err
	}

	newEula := strings.Replace(string(eulafile), "false", "true", -1)
	err = ioutil.WriteFile(eulafilepath, []byte(newEula), 0)
	if err != nil {
		fmt.Println("HandleEula: Replace:", err)
		return err
	}

	return nil
}

func (runner *McRunner) Install() error {
	mcver := "1.12.2"
	forgever := "14.23.5.2836"
	launchwrapperver := "1.12"
	err := runner.InstallForgeJar(mcver, forgever)
	if err != nil {
		fmt.Println("Install: InstallForgeJar:", err)
		return err
	}

	err = runner.InstallLaunchWrapper(launchwrapperver)
	if err != nil {
		fmt.Println("Install: InstallLaunchWrapper:", err)
		return err
	}

	err = runner.InstallMinecraftServerJar(mcver)
	if err != nil {
		fmt.Println("Install: InstallMinecraftServerJar:", err)
		return err
	}

	err = runner.HandleEula()
	if err != nil {
		fmt.Println("Install: HandleEula:", err)
		return err
	}

	return nil
}

// Start initializes the runner and starts the minecraft server up.
func (runner *McRunner) Start() error {
	if runner.State != NotRunning {
		return nil
	}

	if !runner.Installed() {
		fmt.Println("Installing server")
		err := runner.Install()
		if err != nil {
			fmt.Println(err)
			return err
		}
	}
	fmt.Println("Server installed")

	runner.applySettings()
	runner.cmd = exec.Command("java", "-jar", "forge-universal.jar", "-Xms512M", fmt.Sprintf("-Xmx%dM", runner.Settings.MaxRAM), "-XX:+UseG1GC", "-XX:+UseCompressedOops", "-XX:MaxGCPauseMillis=50", "-XX:UseSSE=4", "-XX:+UseNUMA", "nogui")
	runner.cmd.Dir = McServerPath()
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
	propPath := filepath.Join(McServerPath(), "server.properties")
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

			worldPath := filepath.Join(McServerPath(), "world")
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
