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

	currentChallenge string          // the current challenge string for clientsTurn
	usedWords        map[string]bool // the words that have already been used (repeats are not allowed)
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
		clientsTurn: -1,
		usedWords:   make(map[string]bool),
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
		lobby.changeTurn()
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

		lobby.usedWords[answer] = true
		lobby.BroadcastMessage(Message{Type: ANSWER_ACCEPTED, Content: answer})
		lobby.changeTurn()
	}
}

func (lobby *Lobby) changeTurn() {
	// this is definitely not the best way to do this,
	// but it will work fine for the size of our lobbies.
	// the complexity here arises from the fact that the clients in a lobby are not guaranteed to have consecutive ids or start at id 1.
	// since clients can join and leave at any point, a lobby could have ids like the following: [3, 9, 80]
	// the behavior in this case is to start with the lowest id first, then go up through the list, wrapping back around each time
	clientIds := slices.Sorted(maps.Keys(lobby.clients))

	if lobby.clientsTurn == -1 { // if it's the start of the game, choose the oldest client still connected
		lobby.clientsTurn = clientIds[0]
	} else { // otherwise, pick the next client going in order of id (wrapping around as necessary)
		for i, id := range clientIds {
			if id == lobby.clientsTurn {
				lobby.clientsTurn = clientIds[(i+1)%len(lobby.clients)]
				break
			}
		}
	}

	lobby.currentChallenge = words.GetChallenge()
	lobby.BroadcastMessage(Message{Type: CLIENTS_TURN, Content: ClientsTurnContent{ClientId: lobby.clientsTurn, Challenge: lobby.currentChallenge}})
}

func (lobby *Lobby) BroadcastMessage(message Message) {
	for _, client := range lobby.clients {
		client.write <- message
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyOver <- lobby.Id
}
