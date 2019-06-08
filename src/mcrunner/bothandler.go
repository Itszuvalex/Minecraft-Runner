package mcrunner

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

// header defines the header for messages to and from the Discord bot.
type header struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
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

}

// updateStatus frequently sends status updates to the discord bot.
func (handler *BotHandler) updateStatus() {

}
