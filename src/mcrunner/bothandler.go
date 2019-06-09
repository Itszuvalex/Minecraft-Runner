package mcrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// header defines the header for messages to and from the Discord bot.
type header struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// command defines the structure of a command message from the Discord bot.
type command struct {
	Command string
}

// BotHandler encapsulates the communication with the Discord bot.
type BotHandler struct {
	McRunner *McRunner

	sock *websocket.Conn
}

// Start initializes the bot handler and starts up a websocket listener.
func (handler *BotHandler) Start() error {
	var err error
	// Listen for the bot to establish a connection with us.
	s := http.Server{Addr: ":8080", Handler: nil}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade HTTP request to a websocket connection.
		upgrader := websocket.Upgrader{}
		ws, innererr := upgrader.Upgrade(w, r, nil)

		handler.sock = ws
		err = innererr

		// Close http server.
		s.Shutdown(context.Background())

		// Start websocket listener.
		go handler.listen()
	})
	s.ListenAndServe()
	return err
}

// listen listens for messages from the Discord bot.
func (handler *BotHandler) listen() {
	for {
		header := new(header)
		err := handler.sock.ReadJSON(header)

		if err != nil {
			fmt.Println(err)
			break
		}

		switch header.Type {
		case "cmd":
			command := new(command)
			err := json.Unmarshal(header.Data, command)
			if err != nil {
				fmt.Println(err)
				break
			}
			handler.McRunner.CommandChannel <- command.Command
		}
	}
}

// updateStatus frequently sends status updates to the discord bot.
func (handler *BotHandler) updateStatus() {
	for {
		handler.McRunner.StatusRequestChannel <- true

		select {
		case status := <-handler.McRunner.StatusChannel:
			handler.sock.WriteJSON(status)
		case <-time.After(10 * time.Second):
			fmt.Println("Failed to receive status update from runner, might be deadlocked.")
		}

		time.Sleep(60 * time.Second)
	}
}
