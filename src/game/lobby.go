package game

import (
	"github.com/google/uuid"
	"log"
	"sync"
)

type Lobby struct {
	Id        uuid.UUID     // the unique identifier for this lobby
	Join      chan *Client  // channel for new clients to join the lobby
	Leave     chan *Client  // channel for existing clients to leave the lobby
	Broadcast chan *Message // channel for existing clients to broadcast messages through the lobby

	Clients map[int]*Client // all clients in the lobby, indexed by their id

	lastClientId  int        // the id of the last client which connected (used to increment Client.Id's as they join the lobby)
	clientIdMutex sync.Mutex // enforces thread-safe access to the nextClientId
}

func NewLobby() *Lobby {
	return &Lobby{
		Id:        uuid.New(),
		Join:      make(chan *Client),
		Leave:     make(chan *Client),
		Broadcast: make(chan *Message),
		Clients:   make(map[int]*Client),
	}
}

func (lobby *Lobby) GetNextClientId() int {
	lobby.clientIdMutex.Lock()
	defer lobby.clientIdMutex.Unlock()

	lobby.lastClientId++
	return lobby.lastClientId
}

func (lobby *Lobby) StartLobby() {
	for {
		select {
		case client := <-lobby.Join:
			lobby.Clients[client.Id] = client
			log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, client.Id)
		case client := <-lobby.Leave:
			delete(lobby.Clients, client.Id)
			log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, client.Id)
		}
	}
}
