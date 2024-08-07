package game

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
)

type Client struct {
	Id          int             // uniquely identifies the Client within the Lobby
	DisplayName string          // the display name for the client (shown to other players)
	Lobby       *Lobby          // holds a reference to the Lobby that the client is in
	ws          *websocket.Conn // holds a reference to the WebSocket connection
	write       chan Message    // a write channel used by the Lobby to pass messages that the client should transmit over the websocket
}

func JoinClientToLobby(ws *websocket.Conn, lobby *Lobby) error {
	if ws == nil {
		return errors.New("websocket connection must already be established")
	}

	if lobby == nil {
		return errors.New("client must belong to a lobby")
	}

	Id := lobby.GetNextClientId()
	client := &Client{
		Id:          Id,
		DisplayName: fmt.Sprintf("Player %d", Id),
		Lobby:       lobby,
		ws:          ws,
		write:       make(chan Message),
	}

	_ = ws.WriteJSON(Message{Type: CLIENT_ID_ASSIGNED, Content: client.Id})

	lobby.join <- client
	go client.Write()
	go client.Read()

	return nil
}

func (c *Client) Write() {
	defer func() {
		c.Lobby.leave <- c
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
		c.Lobby.leave <- c
		_ = c.ws.Close()
	}()

	for {
		var message Message
		err := c.ws.ReadJSON(&message)
		if err != nil {
			return
		}

		message.From = c.Id
		c.Lobby.read <- message
	}
}
