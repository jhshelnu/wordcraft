package game

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
)

type Client struct {
	id           int             // uniquely identifies the Client within the lobby
	displayName  string          // the display name for the client (shown to other players)
	iconName     string          // the file name of the icon to show for this client in the lobby
	lobby        *Lobby          // holds a reference to the lobby that the client is in
	ws           *websocket.Conn // holds a reference to the WebSocket connection
	write        chan Message    // a write channel used by the lobby to pass messages that the client should transmit over the websocket
	disconnected chan bool       // a channel used by the client's Read and Write goroutines to synchronize disconnects
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
		id:           Id,
		displayName:  fmt.Sprintf("Player %d", Id),
		iconName:     lobby.GetDefaultIconName(Id),
		lobby:        lobby,
		ws:           ws,
		write:        make(chan Message),
		disconnected: make(chan bool),
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

		message.From = c.id
		c.lobby.read <- message
	}
}

func (c *Client) close() {
	c.disconnected <- true // tell the other client goroutine to disconnect
	c.lobby.leave <- c
	_ = c.ws.Close()
}

func (c *Client) String() string {
	return fmt.Sprintf("Client[id=%d, displayName='%s']", c.id, c.displayName)
}
