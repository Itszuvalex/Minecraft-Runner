package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mcrunner"
	"os"
)

func main() {
	fmt.Println("Starting server...")
	runner := new(mcrunner.McRunner)
	runner.Settings = loadSettings()
	go runner.Start()

	bothandler := new(mcrunner.BotHandler)
	bothandler.McRunner = runner
	go bothandler.Start()
}

func loadSettings() mcrunner.Settings {
	_, err := os.Stat("settings.json")
	defaultSettings := mcrunner.Settings{Name: "?", MaxRAM: 6192, MaxPlayers: 20}

	if err != nil {
		file, err := os.Open("settings.json")
		if err == nil {
			bytes, _ := ioutil.ReadAll(file)
			var settings mcrunner.Settings
			json.Unmarshal(bytes, &settings)
			return settings
		}
	} else if os.IsNotExist(err) {
		fmt.Println("'settings.json' not found, generating default file.")
		settingsJSON, _ := json.Marshal(defaultSettings)
		err = ioutil.WriteFile("settings.json", settingsJSON, 0644)
		if err != nil {
			fmt.Print(err)
		}
		return defaultSettings
	}

	fmt.Print(err)
	fmt.Println("Error opening settings file, using defaults.")
	return defaultSettings
}
