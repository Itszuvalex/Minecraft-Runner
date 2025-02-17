package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mcrunner"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	fmt.Println("Starting server...")
	runner := new(mcrunner.McRunner)
	runner.Settings = loadSettings()
	runner.StatusRequestChannel = make(chan bool, 1)
	runner.StatusChannel = make(chan *mcrunner.Status, 1)
	runner.MessageChannel = make(chan string, 32)
	runner.CommandChannel = make(chan string, 32)
	runner.FirstStart = true
	runner.WaitGroup = sync.WaitGroup{}
	go runner.Start()

	bothandler := new(mcrunner.BotHandler)
	bothandler.McRunner = runner
	bothandler.Start()

	// Prevent the program from exiting while there is still stuff running in the background.
	runner.WaitGroup.Wait()
}

func loadSettings() mcrunner.Settings {
	settingspath := filepath.Join(mcrunner.McServerPath(), "settings.json")
	_, err := os.Stat(settingspath)
	defaultSettings := mcrunner.Settings{Directory: "./", Name: "?", MOTD: "?", MaxRAM: 6192, MaxPlayers: 20, Port: 25565, ListenAddress: ":8080", PassthroughStdErr: true, PassthroughStdOut: false}

	if err == nil {
		file, err := os.Open(settingspath)
		if err == nil {
			bytes, _ := ioutil.ReadAll(file)
			var settings mcrunner.Settings
			json.Unmarshal(bytes, &settings)
			return settings
		}
	} else if os.IsNotExist(err) {
		fmt.Println("'settings.json' not found, generating default file.")
		settingsJSON, _ := json.MarshalIndent(defaultSettings, "", "    ")
		err = ioutil.WriteFile(settingspath, settingsJSON, 0644)
		if err != nil {
			fmt.Print(err)
		}
		return defaultSettings
	}

	fmt.Println(err)
	fmt.Println("Error opening settings file, using defaults.")
	return defaultSettings
}
