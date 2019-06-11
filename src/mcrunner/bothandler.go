package mcrunner

import (
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
	Command string `json:"cmd"`
}

// message defines the structure of a message to the Discord bot.
type message struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// BotHandler encapsulates the communication with the Discord bot.
type BotHandler struct {
	McRunner *McRunner

	sock            *websocket.Conn
	killChannel     chan bool
	connectionAlive bool
}

// Start initializes the bot handler and starts up a websocket listener.
func (handler *BotHandler) Start() error {
	handler.killChannel = make(chan bool, 3)
	handler.connectionAlive = false

	// Listen for the bot to establish a connection with us.
	s := http.Server{Addr: handler.McRunner.Settings.ListenAddress, Handler: nil}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade HTTP request to a websocket connection.
		upgrader := websocket.Upgrader{}
		ws, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(fmt.Sprintf("Opened websocket connection from %s.", r.RemoteAddr))

		if handler.connectionAlive {
			handler.sock.Close()
			handler.killChannel <- true
			handler.killChannel <- true
			handler.killChannel <- true

			// Make sure there's time for the kill messages to get through to the old goroutines
			// before we create the new ones.
			time.Sleep(1 * time.Second)
		} else {
			handler.connectionAlive = true
		}

		handler.sock = ws
		handler.sock.SetCloseHandler(func(code int, text string) error {
			message := websocket.FormatCloseMessage(code, "")
			handler.sock.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
			fmt.Println(fmt.Sprintf("Websocket closed with code %d.", code))

			handler.killChannel <- true
			handler.killChannel <- true
			handler.killChannel <- true

			handler.connectionAlive = false

			return nil
		})

		// Start websocket listeners.
		go handler.listen()
		go handler.updateStatus()
		go handler.handleMessages()
	})

	err := s.ListenAndServe()

	if err != nil {
		fmt.Println(err)
	}

	return err
}

// listen listens for messages from the Discord bot.
func (handler *BotHandler) listen() {
	handler.McRunner.WaitGroup.Add(1)
	defer handler.McRunner.WaitGroup.Done()
	for {
		select {
		default:
			header := new(header)
			err := handler.sock.ReadJSON(header)

			if err != nil {
				fmt.Println(err)
				fmt.Println("Killing websocket handler goroutines due to error.")
				handler.killChannel <- true
				handler.killChannel <- true
				handler.killChannel <- true
				handler.sock.Close()
				handler.connectionAlive = false
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
		case <-handler.killChannel:
			return
		}

	}
}

// updateStatus frequently sends status updates to the discord bot.
func (handler *BotHandler) updateStatus() {
	handler.McRunner.WaitGroup.Add(1)
	defer handler.McRunner.WaitGroup.Done()
	for {
		select {
		case <-time.After(60 * time.Second):
			handler.McRunner.StatusRequestChannel <- true

			select {
			case status := <-handler.McRunner.StatusChannel:
				statusJSON, _ := json.Marshal(status)
				header := header{Type: "status", Data: statusJSON}
				handler.sock.WriteJSON(header)
			case <-time.After(10 * time.Second):
				fmt.Println("Failed to receive status update from runner, might be deadlocked.")
			}
		case <-handler.killChannel:
			return
		}

	}
}

// handleMessages forwards chat messages from the mc server to the discord bot.
func (handler *BotHandler) handleMessages() {
	handler.McRunner.WaitGroup.Add(1)
	defer handler.McRunner.WaitGroup.Done()
	for {
		select {
		case msg := <-handler.McRunner.MessageChannel:
			message := message{Timestamp: time.Now().Format(time.RFC3339), Message: msg}
			messageJSON, _ := json.Marshal(message)
			header := header{Type: "msg", Data: messageJSON}
			handler.sock.WriteJSON(header)
		case <-handler.killChannel:
			return
		}
	}
}
