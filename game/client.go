package game

import (
	"errors"
	"github.com/gorilla/websocket"
)

type Client struct {
	Id    int             // uniquely identifies the Client within the Lobby
	Lobby *Lobby          // holds a reference to the Lobby that the client is in
	ws    *websocket.Conn // holds a reference to the WebSocket connection
	write chan Message    // a write channel used by the Lobby to pass messages that the client should transmit over the websocket
}

func JoinClientToLobby(ws *websocket.Conn, lobby *Lobby) error {
	if ws == nil {
		return errors.New("websocket connection must already be established")
	}

	if lobby == nil {
		return errors.New("client must belong to a lobby")
	}

	client := &Client{
		Id:    lobby.GetNextClientId(),
		Lobby: lobby,
		ws:    ws,
		write: make(chan Message),
	}

	lobby.Join <- client
	go client.Write()
	go client.Read()

	return nil
}

func (c *Client) Write() {
	defer func() {
		c.Lobby.Leave <- c
		_ = c.ws.Close()
	}()

	for {
		message, ok := <-c.write
		if !ok {
			_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		err := c.ws.WriteJSON(message)
		if err != nil {
			return
		}
	}
}

func (c *Client) Read() {
	defer func() {
		c.Lobby.Leave <- c
		_ = c.ws.Close()
	}()

	for {
		var message Message
		err := c.ws.ReadJSON(&message)
		if err != nil {
			return
		}

		message.From = c.Id
		c.Lobby.Broadcast <- message
	}
}
