package game

import (
	"errors"
	"github.com/gorilla/websocket"
)

type Client struct {
	Id    int             // uniquely identifies the Client within the Lobby
	Lobby *Lobby          // holds a reference to the Lobby that the client is in
	ws    *websocket.Conn // holds a reference to the WebSocket connection
}

func JoinClientToLobby(ws *websocket.Conn, lobby *Lobby) error {
	if ws == nil {
		return errors.New("websocket connection must already be established")
	}

	if lobby == nil {
		return errors.New("client must belong to a lobby")
	}

	lobby.Join <- &Client{
		Id:    lobby.GetNextClientId(),
		Lobby: lobby,
		ws:    ws,
	}

	return nil
}

// todo: when the ws connection is severed, then broadcast the client to the lobby's Leave channel
