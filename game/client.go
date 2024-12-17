package game

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const reconnectionTimeout = 5 * time.Second

var recoverableWsErrors = []int{websocket.CloseNormalClosure, websocket.CloseGoingAway}

type Client struct {
	id             int             // uniquely identifies the Client within the lobby
	reconnectToken string          // a randomly generated token sent to the client to be used for reconnecting
	displayName    string          // the display name for the client (shown to other players)
	iconName       string          // the file name of the icon to show for this client in the lobby
	lobby          *Lobby          // holds a reference to the lobby that the client is in
	ws             *websocket.Conn // holds a reference to the WebSocket connection
	wsMut          sync.Mutex      // used to synchronize clearing and re-establishing new websocket conns between client threads
	write          chan Message    // a write channel used by the lobby to pass messages that the client should transmit over the websocket
	disconnected   chan bool       // a channel used by the client's Read and Write goroutines to synchronize disconnects
}

// JoinConnToLobby registers this connection as belonging to a client in the lobby
// either by creating a new client and connecting, or by re-establishing connection with an existing client (using the specified reconnectToken if not empty)
// If the (non-empty) reconnectToken is not valid for the current lobby, a new client will be established
func JoinConnToLobby(ws *websocket.Conn, lobby *Lobby, reconnectToken string) error {
	if ws == nil {
		return errors.New("websocket connection must already be established")
	}

	if lobby == nil {
		return errors.New("client must belong to a lobby")
	}

	// attempt to reconnect to an existing client if we have a reconnectToken that matches one of an existingClient
	if existingClient := lobby.GetClientByReconnectToken(reconnectToken); existingClient != nil {
		if existingClient.ws == nil {
			existingClient.ws = ws
			return nil
		} else {
			return errors.New("client is already present in the lobby")
		}
	}

	Id := lobby.GetNextClientId()
	client := &Client{
		id:             Id,
		reconnectToken: generateReconnectToken(),
		displayName:    fmt.Sprintf("Player %d", Id),
		iconName:       lobby.GetDefaultIconName(Id),
		lobby:          lobby,
		ws:             ws,
		wsMut:          sync.Mutex{},
		write:          make(chan Message),
		disconnected:   make(chan bool),
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
			c.wsMut.Lock()
			if c.ws != nil {
				_ = c.ws.WriteJSON(message)
			}
			c.wsMut.Unlock()
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

		// no connection issue
		if err == nil {
			message.From = c.id
			c.lobby.read <- message
			continue
		}

		// unrecoverable connection issue
		if !isRecoverableWsError(err) {
			c.lobby.logger.Printf("%s has disconnected. Won't wait for reconnection due to: %v", c, err)
			return
		}

		// recoverable connection issue
		c.wsMut.Lock()
		_ = c.ws.Close()
		c.ws = nil
		c.wsMut.Unlock()

		c.lobby.logger.Printf("%s has disconnected. Waiting for reconnection...", c)
		if c.awaitRecovery() {
			c.lobby.logger.Printf("%s has reconnected", c)
			c.lobby.read <- Message{Type: ClientDetailsReq, From: c.id} // ask the server for a full catch-up of what's been missed
		} else {
			c.lobby.logger.Printf("%s was not able to recover their connection in time", c)
			return
		}
	}
}

// awaitRecovery returns true if the client connection has been recovered, or false if the connection took too long to be recoverd
func (c *Client) awaitRecovery() bool {
	ticker := time.Tick(50 * time.Millisecond)
	timeout := time.After(reconnectionTimeout)
	for {
		select {
		case <-ticker:
			if c.ws != nil {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

func (c *Client) close() {
	if r := recover(); r != nil {
		fmt.Printf("Client.close() recovered from: %v\n", r)
	}

	c.disconnected <- true // tell the other client goroutine to disconnect
	c.lobby.leave <- c     // tell the lobby we've left

	if c.ws != nil {
		_ = c.ws.Close()
	}
}

func (c *Client) String() string {
	return fmt.Sprintf("Client[id=%d, displayName='%s']", c.id, c.displayName)
}

func isRecoverableWsError(err error) bool {
	return websocket.IsCloseError(err, recoverableWsErrors...)
}

func generateReconnectToken() string {
	tokenBytes := make([]byte, 32)

	_, err := rand.Read(tokenBytes)
	if err != nil {
		panic(fmt.Sprintf("unable to generate reconnect token: %v", err))
	}

	return hex.EncodeToString(tokenBytes)
}
