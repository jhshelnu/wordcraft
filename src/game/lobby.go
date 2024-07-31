package game

import (
	"github.com/google/uuid"
	"log"
	"sync"
)

type Lobby struct {
	Id        uuid.UUID    // the unique identifier for this lobby
	Join      chan *Client // channel for new clients to join the lobby
	Leave     chan *Client // channel for existing clients to leave the lobby
	Broadcast chan Message // channel for existing clients to broadcast messages through the lobby

	Clients map[int]*Client // all clients in the lobby, indexed by their id

	lastClientId  int        // the id of the last client which connected (used to increment Client.Id's as they join the lobby)
	clientIdMutex sync.Mutex // enforces thread-safe access to the nextClientId

	lobbyOver chan uuid.UUID // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed
}

func NewLobby(lobbyOver chan uuid.UUID) *Lobby {
	return &Lobby{
		Id:        uuid.New(),
		Join:      make(chan *Client),
		Leave:     make(chan *Client),
		Broadcast: make(chan Message),
		Clients:   make(map[int]*Client),
		lobbyOver: lobbyOver,
	}
}

func (lobby *Lobby) GetNextClientId() int {
	lobby.clientIdMutex.Lock()
	defer lobby.clientIdMutex.Unlock()

	lobby.lastClientId++
	return lobby.lastClientId
}

func (lobby *Lobby) StartLobby() {
	defer lobby.EndLobby()

	for {
		select {
		case client := <-lobby.Join:
			lobby.Clients[client.Id] = client
			log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, client.Id)
		case client := <-lobby.Leave:
			delete(lobby.Clients, client.Id)
			log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, client.Id)
			if len(lobby.Clients) == 0 {
				log.Printf("[Lobby %s] All clients have disconnected. Game over.\n", lobby.Id)
				return
			}
		case message := <-lobby.Broadcast:
			lobby.HandleMessage(message)
		}
	}
}

func (lobby *Lobby) HandleMessage(message Message) {
	log.Printf("[Lobby %s] Client %d sent: %s\n", lobby.Id, message.From, message.Content)
	for _, client := range lobby.Clients {
		if client.Id != message.From {
			client.write <- message
		}
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyOver <- lobby.Id
}
