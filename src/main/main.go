package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mcrunner"
	"os"
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
	runner.WaitGroup.Add(4)
	go runner.Start()

	bothandler := new(mcrunner.BotHandler)
	bothandler.McRunner = runner
	runner.WaitGroup.Add(3)
	go bothandler.Start()

	runner.WaitGroup.Wait()
}

func loadSettings() mcrunner.Settings {
	_, err := os.Stat("settings.json")
	defaultSettings := mcrunner.Settings{Directory: "./", Name: "?", MOTD: "?", MaxRAM: 6192, MaxPlayers: 20, Port: 25565}

	if err == nil {
		file, err := os.Open("settings.json")
		if err == nil {
			bytes, _ := ioutil.ReadAll(file)
			var settings mcrunner.Settings
			json.Unmarshal(bytes, &settings)
			return settings
		}
	} else if os.IsNotExist(err) {
		fmt.Println("'settings.json' not found, generating default file.")
		settingsJSON, _ := json.MarshalIndent(defaultSettings, "", "    ")
		err = ioutil.WriteFile("settings.json", settingsJSON, 0644)
		if err != nil {
			fmt.Print(err)
		}
		return defaultSettings
	}

	fmt.Println(err)
	fmt.Println("Error opening settings file, using defaults.")
	return defaultSettings
}
