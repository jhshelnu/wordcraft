package game

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
)

type Client struct {
	Id           int             // uniquely identifies the Client within the Lobby
	DisplayName  string          // the display name for the client (shown to other players)
	IconName     string          // the file name of the icon to show for this client in the lobby
	Lobby        *Lobby          // holds a reference to the Lobby that the client is in
	ws           *websocket.Conn // holds a reference to the WebSocket connection
	write        chan Message    // a write channel used by the Lobby to pass messages that the client should transmit over the websocket
	disconnected chan any        // a channel used by the client's read and write goroutines to synchronize disconnects
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
		Id:           Id,
		DisplayName:  fmt.Sprintf("Player %d", Id),
		IconName:     lobby.GetDefaultIconName(Id),
		Lobby:        lobby,
		ws:           ws,
		write:        make(chan Message),
		disconnected: make(chan any),
	}

	go client.Write()
	go client.Read()
	lobby.join <- client

	return nil
}

func (c *Client) Write() {
	defer c.close()

	for {
		select {
		case message := <-c.write:
			err := c.ws.WriteJSON(message)
			if err != nil {
				return
			}
		case <-c.disconnected:
			return
		}
	}
}

func (c *Client) Read() {
	defer c.close()

	for {
		// check if we've disconnected without blocking
		select {
		case <-c.disconnected:
			return
		default:
			// nothing to do
		}

		var message Message
		err := c.ws.ReadJSON(&message)
		if err != nil {
			return
		}

		message.From = c.Id
		c.Lobby.read <- message
	}
}

func (c *Client) close() {
	c.disconnected <- true // tell the other client goroutine to disconnect
	c.Lobby.leave <- c
	_ = c.ws.Close()
}

func (c *Client) String() string {
	return fmt.Sprintf("Client[Id=%d, DisplayName='%s']", c.Id, c.DisplayName)
}
