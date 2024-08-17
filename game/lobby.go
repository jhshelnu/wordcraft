package game

import (
	"github.com/google/uuid"
	"github.com/jhshelnu/wordgame/icons"
	"github.com/jhshelnu/wordgame/words"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	TURN_LIMIT_SECONDS = 20
	MAX_DISPLAY_NAME   = 15
)

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

	iconNames []string // a slice of icon names (shuffled for each lobby)

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
		iconNames: icons.GetShuffledIconNames(),
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

func (lobby *Lobby) GetDefaultIconName(id int) string {
	return lobby.iconNames[(id-1)%len(lobby.iconNames)]
}

func (lobby *Lobby) GetClients() []*Client {
	return slices.SortedFunc(maps.Values(lobby.clients), func(client1 *Client, client2 *Client) int { return client1.Id - client2.Id })
}

func (lobby *Lobby) StartLobby() {
	defer lobby.EndLobby()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Lobby %s] Encountered fatal error: %v\n", lobby.Id, r)
		}
	}()

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
	if lobby.Status == WAITING_FOR_PLAYERS {
		lobby.aliveClients = append(lobby.aliveClients, joiningClient)
	}

	lobby.BroadcastMessage(Message{Type: CLIENT_JOINED, Content: ClientJoinedContent{
		ClientId:    joiningClient.Id,
		DisplayName: joiningClient.DisplayName,
		IconName:    joiningClient.IconName,
	}})

	log.Printf("[Lobby %s] Client %d connected\n", lobby.Id, joiningClient.Id)
}

func (lobby *Lobby) onClientLeave(leavingClient *Client) {
	log.Printf("[Lobby %s] Client %d disconnected\n", lobby.Id, leavingClient.Id)
	delete(lobby.clients, leavingClient.Id)
	lobby.BroadcastMessage(Message{Type: CLIENT_LEFT, Content: leavingClient.Id})

	// not very efficient, but there won't be many clients anyway
	if lobby.Status == IN_PROGRESS {
		aliveClients := make([]*Client, 0, len(lobby.aliveClients)-1)
		for _, c := range lobby.aliveClients {
			if c.Id != leavingClient.Id {
				aliveClients = append(aliveClients, c)
			}
		}
		lobby.aliveClients = aliveClients
	}

	if len(lobby.aliveClients) == 1 {
		// only one client left, we have a winner i guess
		lobby.Status = OVER
		lobby.BroadcastMessage(Message{Type: GAME_OVER})
	} else {
		lobby.changeTurn(true)
	}
}

func (lobby *Lobby) onMessage(message Message) {
	switch message.Type {
	case START_GAME:
		lobby.onStartGame()
	case RESTART_GAME:
		lobby.onRestartGame()
	case ANSWER_PREVIEW:
		lobby.onAnswerPreview(message)
	case SUBMIT_ANSWER:
		lobby.onAnswerSubmitted(message)
	case NAME_CHANGE:
		lobby.onNameChange(message)
	}
}

func (lobby *Lobby) onTurnExpired() {
	lobby.BroadcastMessage(Message{Type: TURN_EXPIRED, Content: lobby.aliveClients[lobby.turnIndex].Id})
	if len(lobby.aliveClients) > 2 {
		// at least 2 clients still alive, keep the game going (lobby#changeTurn will handle dropping them)
		lobby.changeTurn(true)
	} else {
		// only one client alive, we have a winner
		lobby.Status = OVER

		// we're here because there are 2 clients remaining and one of them just had their turn expire
		// so, the winner is the *other* one
		var winningClientId int
		if lobby.turnIndex == 0 {
			winningClientId = lobby.aliveClients[1].Id
		} else {
			winningClientId = lobby.aliveClients[0].Id
		}

		lobby.BroadcastMessage(Message{Type: GAME_OVER, Content: winningClientId})
	}
}

func (lobby *Lobby) onStartGame() {
	if lobby.Status == WAITING_FOR_PLAYERS && len(lobby.clients) >= 2 {
		lobby.Status = IN_PROGRESS
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) onRestartGame() {
	if lobby.Status == OVER {
		// reset alive clients to hold all clients
		lobby.aliveClients = slices.SortedFunc(maps.Values(lobby.clients), func(c1 *Client, c2 *Client) int {
			return c1.Id - c2.Id
		})

		lobby.Status = IN_PROGRESS
		lobby.turnIndex = -1
		lobby.BroadcastMessage(Message{Type: RESTART_GAME})
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) onNameChange(message Message) {
	if lobby.Status == WAITING_FOR_PLAYERS {
		newDisplayName, ok := message.Content.(string)
		if !ok || len(newDisplayName) > MAX_DISPLAY_NAME {
			return
		}

		client := lobby.clients[message.From]
		client.DisplayName = newDisplayName
		lobby.BroadcastMessage(Message{Type: NAME_CHANGE, Content: ClientNameChange{ClientId: client.Id, NewDisplayName: newDisplayName}})
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

		if !words.IsValidWord(answer) || !strings.Contains(answer, lobby.currentChallenge) || lobby.usedWords[answer] {
			lobby.BroadcastMessage(Message{Type: ANSWER_REJECTED, Content: answer})
			return
		}

		lobby.usedWords[answer] = true
		lobby.BroadcastMessage(Message{Type: ANSWER_ACCEPTED, Content: answer})
		lobby.changeTurn(false)
	}
}

// removeCurrentClient indicates if the client (whose turn it is) has gone out
// this can happen either by time running out, or by the client disconnecting
// regardless, it is the responsibility of this method to properly update the aliveClients and turnIndex variables
func (lobby *Lobby) changeTurn(removeCurrentClient bool) {
	if !removeCurrentClient {
		// if the last client didn't run out of time or disconnect, this is easy
		lobby.turnIndex = (lobby.turnIndex + 1) % len(lobby.aliveClients)
	} else {
		// if they ran out of time or disconnected:
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
