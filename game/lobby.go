package game

import (
	"github.com/google/uuid"
	"github.com/jhshelnu/wordgame/words"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

const TURN_LIMIT_SECONDS = 12

type gameStatus int

const (
	WAITING_FOR_PLAYERS = iota
	IN_PROGRESS
	OVER
)

type Lobby struct {
	Id    uuid.UUID    // the unique identifier for this lobby
	join  chan *Client // channel for new clients to join the lobby
	leave chan *Client // channel for existing clients to leave the lobby
	read  chan Message // channel for existing clients to send messages for the Lobby to read

	Status       gameStatus      // the Status of the game, indicates if its started, in progress, etc
	clients      map[int]*Client // all clients in the lobby, indexed by their id
	aliveClients []*Client       // all clients in the lobby who are not out
	turnIndex    int             // the index in aliveClients of whose turn it is

	lastClientId int // the id of the last client which connected (used to increment Client.Id's as they join the lobby)

	clientIdMutex sync.Mutex     // enforces thread-safe access to the nextClientId
	lobbyOver     chan uuid.UUID // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed

	currentChallenge string           // the current challenge string for clientsTurn
	usedWords        map[string]bool  // the words that have already been used (repeats are not allowed)
	turnExpired      <-chan time.Time // a (read-only) channel which produces a single boolean value once the client has run out of time
}

func NewLobby(lobbyOver chan uuid.UUID) *Lobby {
	return &Lobby{
		Id:        uuid.New(),
		join:      make(chan *Client),
		leave:     make(chan *Client),
		read:      make(chan Message),
		Status:    WAITING_FOR_PLAYERS,
		clients:   make(map[int]*Client),
		turnIndex: -1,
		lobbyOver: lobbyOver,
		usedWords: make(map[string]bool),
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
			lobby.onClientJoin(client)
		case client := <-lobby.leave:
			lobby.onClientLeave(client)
			if len(lobby.clients) == 0 {
				log.Printf("[Lobby %s] All clients have disconnected. Game over.\n", lobby.Id)
				return
			}
		case message := <-lobby.read:
			lobby.onMessage(message)
		case <-lobby.turnExpired:
			lobby.onTurnExpired()
		}
	}
}

func (lobby *Lobby) onClientJoin(joiningClient *Client) {
	lobby.clients[joiningClient.Id] = joiningClient
	lobby.aliveClients = append(lobby.aliveClients, joiningClient)
	lobby.BroadcastMessage(Message{Type: CLIENT_JOINED, Content: joiningClient.Id})
	log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, joiningClient.Id)
}

func (lobby *Lobby) onClientLeave(leavingClient *Client) {
	delete(lobby.clients, leavingClient.Id)

	// not very efficient, but there won't be many clients anyway
	aliveClients := make([]*Client, 0, len(lobby.aliveClients)-1)
	for _, c := range lobby.aliveClients {
		if c.Id != leavingClient.Id {
			aliveClients = append(aliveClients, c)
		}
	}
	lobby.aliveClients = aliveClients

	lobby.BroadcastMessage(Message{Type: CLIENT_LEFT, Content: leavingClient.Id})
	log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, leavingClient.Id)
}

func (lobby *Lobby) onMessage(message Message) {
	switch message.Type {
	case START_GAME:
		lobby.onStartGame()
	case ANSWER_PREVIEW:
		lobby.onAnswerPreview(message)
	case SUBMIT_ANSWER:
		lobby.onAnswerSubmitted(message)
	}
}

func (lobby *Lobby) onTurnExpired() {
	lobby.BroadcastMessage(Message{Type: TURN_EXPIRED})
	if len(lobby.aliveClients) > 2 {
		// at least 2 clients still alive, keep the game going (lobby#changeTurn will handle dropping them)
		lobby.changeTurn(true)
	} else {
		// only one client alive, we have a winner
		lobby.Status = OVER
		lobby.BroadcastMessage(Message{Type: GAME_OVER})
	}
}

func (lobby *Lobby) onStartGame() {
	if lobby.Status == WAITING_FOR_PLAYERS && len(lobby.clients) >= 2 {
		lobby.Status = IN_PROGRESS
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) onAnswerPreview(message Message) {
	if lobby.Status == IN_PROGRESS && message.From == lobby.aliveClients[lobby.turnIndex].Id {
		lobby.BroadcastMessage(message)
	}
}

func (lobby *Lobby) onAnswerSubmitted(message Message) {
	if lobby.Status == IN_PROGRESS && message.From == lobby.aliveClients[lobby.turnIndex].Id {
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
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) changeTurn(turnExpired bool) {
	if !turnExpired {
		// if the last client didn't run out of time, this is easy
		lobby.turnIndex = (lobby.turnIndex + 1) % len(lobby.aliveClients)
	} else {
		// if they did run out of time:
		// - kick them out of the aliveClients
		// - turnIndex can stay the same (since the next client will now occupy that index)
		//   unless the last client got eliminated, in which case just need to reset the turnIndex to 0
		aliveClients := make([]*Client, 0, len(lobby.aliveClients)-1)
		for _, c := range lobby.aliveClients {
			if c.Id != lobby.aliveClients[lobby.turnIndex].Id {
				aliveClients = append(aliveClients, c)
			}
		}
		lobby.aliveClients = aliveClients

		if lobby.turnIndex == len(lobby.aliveClients) {
			lobby.turnIndex = 0
		}
	}

	lobby.currentChallenge = words.GetChallenge()
	lobby.BroadcastMessage(Message{Type: CLIENTS_TURN, Content: ClientsTurnContent{ClientId: lobby.aliveClients[lobby.turnIndex].Id, Challenge: lobby.currentChallenge}})
	lobby.turnExpired = time.After(TURN_LIMIT_SECONDS * time.Second)
}

func (lobby *Lobby) BroadcastMessage(message Message) {
	for _, client := range lobby.clients {
		client.write <- message
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyOver <- lobby.Id
}
