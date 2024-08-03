package game

import (
	"github.com/google/uuid"
	"github.com/jhshelnu/wordgame/words"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
)

type gameStatus int

const (
	WAITING_FOR_PLAYERS = iota
	IN_PROGRESS
	GAME_OVER
)

type Lobby struct {
	Id    uuid.UUID    // the unique identifier for this lobby
	join  chan *Client // channel for new clients to join the lobby
	leave chan *Client // channel for existing clients to leave the lobby
	read  chan Message // channel for existing clients to send messages for the Lobby to read

	clients map[int]*Client // all clients in the lobby, indexed by their id

	lastClientId  int        // the id of the last client which connected (used to increment Client.Id's as they join the lobby)
	clientIdMutex sync.Mutex // enforces thread-safe access to the nextClientId

	lobbyOver chan uuid.UUID // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed

	Status      gameStatus // the Status of the game, indicates if its started, in progress, etc
	clientsTurn int        // the id of the client whose turn it is (if applicable)

	currentChallenge string // the current challenge string for clientsTurn
}

func NewLobby(lobbyOver chan uuid.UUID) *Lobby {
	return &Lobby{
		Id:          uuid.New(),
		join:        make(chan *Client),
		leave:       make(chan *Client),
		read:        make(chan Message),
		clients:     make(map[int]*Client),
		lobbyOver:   lobbyOver,
		Status:      WAITING_FOR_PLAYERS,
		clientsTurn: 1,
	}
}

func (lobby *Lobby) GetNextClientId() int {
	lobby.clientIdMutex.Lock()
	defer lobby.clientIdMutex.Unlock()

	lobby.lastClientId++
	return lobby.lastClientId
}

func (lobby *Lobby) GetClientIds() []int {
	return slices.Sorted(maps.Keys(lobby.clients))
}

func (lobby *Lobby) StartLobby() {
	defer lobby.EndLobby()
	for {
		select {
		case client := <-lobby.join:
			lobby.HandleClientJoin(client)
		case client := <-lobby.leave:
			lobby.HandleClientLeave(client)
			if len(lobby.clients) == 0 {
				log.Printf("[Lobby %s] All clients have disconnected. Game over.\n", lobby.Id)
				return
			}
		case message := <-lobby.read:
			lobby.HandleMessage(message)
		}
	}
}

func (lobby *Lobby) HandleClientJoin(client *Client) {
	lobby.clients[client.Id] = client
	lobby.BroadcastMessage(Message{Type: CLIENT_JOINED, Content: client.Id})
	log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, client.Id)
}

func (lobby *Lobby) HandleClientLeave(client *Client) {
	delete(lobby.clients, client.Id)
	lobby.BroadcastMessage(Message{Type: CLIENT_LEFT, Content: client.Id})
	log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, client.Id)
}

func (lobby *Lobby) HandleMessage(message Message) {
	switch message.Type {
	case START_GAME:
		lobby.onStartGame()
	case ANSWER_PREVIEW:
		lobby.onAnswerPreview(message)
	case SUBMIT_ANSWER:
		lobby.onAnswerSubmitted(message)
	}
}

func (lobby *Lobby) onStartGame() {
	if lobby.Status == WAITING_FOR_PLAYERS {
		lobby.Status = IN_PROGRESS
		lobby.clientsTurn = 1
		lobby.BroadcastMessage(Message{Type: CLIENTS_TURN, Content: ClientsTurnContent{ClientId: lobby.clientsTurn, Challenge: words.GetChallenge()}})
	}
}

func (lobby *Lobby) onAnswerPreview(message Message) {
	if lobby.Status == IN_PROGRESS && message.From == lobby.clientsTurn {
		lobby.BroadcastMessage(message)
	}
}

func (lobby *Lobby) onAnswerSubmitted(message Message) {
	if lobby.Status == IN_PROGRESS && message.From == lobby.clientsTurn {
		answer, ok := message.Content.(string)
		if !ok {
			return
		}

		if !words.IsValidWord(answer) || !strings.Contains(answer, lobby.currentChallenge) {
			lobby.BroadcastMessage(Message{Type: ANSWER_REJECTED, Content: answer})
			return
		}

		lobby.BroadcastMessage(Message{Type: ANSWER_ACCEPTED, Content: answer})
		lobby.clientsTurn = (lobby.clientsTurn % len(lobby.clients)) + 1
		lobby.BroadcastMessage(Message{Type: CLIENTS_TURN, Content: ClientsTurnContent{ClientId: lobby.clientsTurn, Challenge: words.GetChallenge()}})
	}
}

func (lobby *Lobby) BroadcastMessage(message Message) {
	for _, client := range lobby.clients {
		client.write <- message
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyOver <- lobby.Id
}
