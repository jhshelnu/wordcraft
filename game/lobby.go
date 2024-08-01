package game

import (
	"github.com/google/uuid"
	"log"
	"strconv"
	"sync"
)

type gameStatus int

const (
	WAITING_FOR_PLAYERS = iota
	IN_PROGRESS
	GAME_OVER
)

type Lobby struct {
	Id        uuid.UUID    // the unique identifier for this lobby
	join      chan *Client // channel for new clients to join the lobby
	leave     chan *Client // channel for existing clients to leave the lobby
	broadcast chan Message // channel for existing clients to broadcast messages through the lobby

	Clients map[int]*Client // all clients in the lobby, indexed by their id

	lastClientId  int        // the id of the last client which connected (used to increment Client.Id's as they join the lobby)
	clientIdMutex sync.Mutex // enforces thread-safe access to the nextClientId

	lobbyOver chan uuid.UUID // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed

	status      gameStatus // the status of the game, indicates if its started, in progress, etc
	clientsTurn int        // the id of the client whose turn it is (if applicable)
}

func NewLobby(lobbyOver chan uuid.UUID) *Lobby {
	return &Lobby{
		Id:          uuid.New(),
		join:        make(chan *Client),
		leave:       make(chan *Client),
		broadcast:   make(chan Message),
		Clients:     make(map[int]*Client),
		lobbyOver:   lobbyOver,
		status:      WAITING_FOR_PLAYERS,
		clientsTurn: 1,
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
		case client := <-lobby.join:
			lobby.HandleClientJoin(client)
		case client := <-lobby.leave:
			lobby.HandleClientLeave(client)
			if len(lobby.Clients) == 0 {
				log.Printf("[Lobby %s] All clients have disconnected. Game over.\n", lobby.Id)
				return
			}
		case message := <-lobby.broadcast:
			lobby.HandleMessage(message)
		}
	}
}

func (lobby *Lobby) HandleClientJoin(client *Client) {
	lobby.Clients[client.Id] = client
	lobby.BroadcastMessage(Message{Type: CLIENT_JOINED, Arg: strconv.Itoa(client.Id)})
	log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, client.Id)
}

func (lobby *Lobby) HandleClientLeave(client *Client) {
	delete(lobby.Clients, client.Id)
	lobby.BroadcastMessage(Message{Type: CLIENT_LEFT, Arg: strconv.Itoa(client.Id)})
	log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, client.Id)
}

func (lobby *Lobby) HandleMessage(message Message) {
	log.Printf("[Lobby %s] In state %d, received message: %+v\n", lobby.Id, lobby.status, message)
	switch message.Type {
	case START_GAME:
		lobby.status = IN_PROGRESS
		lobby.clientsTurn = 1
		lobby.BroadcastMessage(Message{Type: CLIENTS_TURN, Arg: strconv.Itoa(lobby.clientsTurn)})
	}
}

func (lobby *Lobby) BroadcastMessage(message Message) {
	for _, client := range lobby.Clients {
		client.write <- message
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyOver <- lobby.Id
}
