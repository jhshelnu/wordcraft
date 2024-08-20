package game

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/jhshelnu/wordgame/icons"
	"github.com/jhshelnu/wordgame/words"
	"log"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	TURN_LIMIT_SECONDS = 20
	MAX_DISPLAY_NAME   = 15
)

//go:generate stringer -type gameStatus
type gameStatus int

const (
	WAITING_FOR_PLAYERS gameStatus = iota
	IN_PROGRESS
	OVER
)

type Lobby struct {
	logger *log.Logger

	Id    uuid.UUID    // the unique identifier for this lobby
	join  chan *Client // channel for new clients to join the lobby
	leave chan *Client // channel for existing clients to leave the lobby
	read  chan Message // channel for existing clients to send messages for the Lobby to read

	iconNames []string // a slice of icon file names (shuffled for each lobby)

	Status       gameStatus      // the Status of the game, indicates if its started, in progress, etc
	clients      map[int]*Client // all clients in the lobby, indexed by their id
	aliveClients []*Client       // all clients in the lobby who are not out
	turnIndex    int             // the index in aliveClients of whose turn it is

	lastClientId int // the id of the last client which connected (used to increment Client.Id's as they join the lobby)

	clientIdMutex sync.Mutex     // enforces thread-safe access to the nextClientId
	lobbyOver     chan uuid.UUID // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed

	currentChallenge string           // the current challenge string for clientsTurn
	turnExpired      <-chan time.Time // a (read-only) channel which produces a single boolean value once the client has run out of time
}

func NewLobby(lobbyOver chan uuid.UUID) *Lobby {
	Id := uuid.New()
	logger := log.New(os.Stdout, fmt.Sprintf("Lobby [%s]: ", Id), log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)

	return &Lobby{
		logger:    logger,
		Id:        Id,
		join:      make(chan *Client),
		leave:     make(chan *Client),
		read:      make(chan Message),
		iconNames: icons.GetShuffledIconNames(),
		Status:    WAITING_FOR_PLAYERS,
		clients:   make(map[int]*Client),
		turnIndex: -1,
		lobbyOver: lobbyOver,
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
			lobby.logger.Printf("Encountered fatal error: %v\n%s", r, debug.Stack())
		}
	}()

	for {
		select {
		case client := <-lobby.join:
			lobby.onClientJoin(client)
		case client := <-lobby.leave:
			lobby.onClientLeave(client)
			if len(lobby.clients) == 0 {
				lobby.logger.Printf("All clients have disconnected. Goodbye.")
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
	lobby.logger.Printf("%s connected", joiningClient)

	lobby.clients[joiningClient.Id] = joiningClient

	// todo: send more than just the clientid, they also need to know gamestate, who's out, who's turn it is, etc
	//       ideally we'd also send things like current player names and pictures, to prevent missing messages while connecting
	joiningClient.write <- Message{Type: CLIENT_DETAILS, Content: ClientDetailsContent{
		ClientId: joiningClient.Id,
		Status:   lobby.Status,
	}}

	lobby.BroadcastMessage(Message{Type: CLIENT_JOINED, Content: ClientJoinedContent{
		ClientId:    joiningClient.Id,
		DisplayName: joiningClient.DisplayName,
		IconName:    joiningClient.IconName,
	}})
}

func (lobby *Lobby) onClientLeave(leavingClient *Client) {
	lobby.logger.Printf("%s disconnected", leavingClient)

	delete(lobby.clients, leavingClient.Id)
	lobby.BroadcastMessage(Message{Type: CLIENT_LEFT, Content: leavingClient.Id})

	// the rest of the code in here is concerned with leaving aliveClients in a consistent state
	// if the game isn't currently in progress or the leaving client is already eliminated, then there is nothing left to do
	if lobby.Status != IN_PROGRESS || !slices.Contains(lobby.aliveClients, leavingClient) {
		return
	}

	// handle game end based on leaving
	if len(lobby.aliveClients) == 2 {
		// only one client alive, we have a winner
		lobby.Status = OVER

		// we're here because there are 2 clients remaining and one of them just left
		// so, the winner is the *other* one
		var winningClient *Client
		if lobby.aliveClients[0] == leavingClient {
			winningClient = lobby.aliveClients[1]
		} else {
			winningClient = lobby.aliveClients[0]
		}

		lobby.logger.Printf("Set the status to %s because %s left, which makes %s the winner", lobby.Status, leavingClient, winningClient)

		lobby.BroadcastMessage(Message{Type: GAME_OVER, Content: winningClient.Id})
		return
	}

	// if a client leaves during their turn, remove them from the aliveClients list, and change the turn to the next client
	leavingClientTurnIndex := slices.Index(lobby.aliveClients, leavingClient)
	if leavingClientTurnIndex == lobby.turnIndex {
		lobby.logger.Printf("Changing the current turn because %s left while it was their turn", leavingClient)
		lobby.changeTurn(true)
		return
	}

	// if it's not their turn, no need to change the turn. can go ahead and remove them from aliveClients
	aliveClients := make([]*Client, 0, len(lobby.aliveClients)-1)
	for _, c := range lobby.aliveClients {
		if c.Id != leavingClient.Id {
			aliveClients = append(aliveClients, c)
		}
	}
	lobby.aliveClients = aliveClients

	// ensure turnIndex stays pointed at the same client
	if leavingClientTurnIndex < lobby.turnIndex {
		lobby.turnIndex--
	}
}

func (lobby *Lobby) onMessage(message Message) {
	switch message.Type {
	case START_GAME:
		lobby.onStartGame(message)
	case RESTART_GAME:
		lobby.onRestartGame(message)
	case ANSWER_PREVIEW:
		lobby.onAnswerPreview(message)
	case SUBMIT_ANSWER:
		lobby.onAnswerSubmitted(message)
	case NAME_CHANGE:
		lobby.onNameChange(message)
	default:
		lobby.logger.Printf("Received message with type %s. Ignoring due to no handler function", message.Type)
	}
}

func (lobby *Lobby) onTurnExpired() {
	// sometimes, depending on timing, our timer can fire after the players have left
	if lobby.Status != IN_PROGRESS {
		lobby.logger.Printf("Ignoring %s message because lobby is in %s status", TURN_EXPIRED, lobby.Status)
		return
	}

	lobby.BroadcastMessage(Message{Type: TURN_EXPIRED, Content: lobby.aliveClients[lobby.turnIndex].Id})
	if len(lobby.aliveClients) > 2 {
		// at least 2 clients still alive, keep the game going (lobby#changeTurn will handle dropping them)
		lobby.changeTurn(true)
	} else {
		// only one client alive, we have a winner
		lobby.Status = OVER

		// we're here because there are 2 clients remaining and one of them just had their turn expire
		// so, the winner is the *other* one
		var winningClient *Client
		if lobby.turnIndex == 0 {
			winningClient = lobby.aliveClients[1]
		} else {
			winningClient = lobby.aliveClients[0]
		}

		lobby.logger.Printf("Set the status to %s because %s ran out of time, which makes %s the winner",
			lobby.Status, lobby.aliveClients[lobby.turnIndex], winningClient)

		lobby.BroadcastMessage(Message{Type: GAME_OVER, Content: winningClient.Id})
	}
}

func (lobby *Lobby) onStartGame(message Message) {
	if lobby.Status == WAITING_FOR_PLAYERS && len(lobby.clients) >= 2 {
		lobby.logger.Printf("%s has started the game", lobby.clients[message.From])
		lobby.Status = IN_PROGRESS
		lobby.resetAliveClients()
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) onRestartGame(message Message) {
	if lobby.Status == OVER && len(lobby.clients) >= 2 {
		lobby.logger.Printf("%s has restarted the game", lobby.clients[message.From])
		lobby.resetAliveClients()
		lobby.Status = IN_PROGRESS
		lobby.turnIndex = -1
		lobby.BroadcastMessage(Message{Type: RESTART_GAME})
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) resetAliveClients() {
	// reset alive clients to hold all clients
	lobby.aliveClients = slices.SortedFunc(maps.Values(lobby.clients), func(c1 *Client, c2 *Client) int {
		return c1.Id - c2.Id
	})
}

func (lobby *Lobby) onNameChange(message Message) {
	newDisplayName, ok := message.Content.(string)
	if !ok || len(newDisplayName) > MAX_DISPLAY_NAME {
		return
	}

	client := lobby.clients[message.From]
	client.DisplayName = newDisplayName
	lobby.BroadcastMessage(Message{Type: NAME_CHANGE, Content: ClientNameChange{ClientId: client.Id, NewDisplayName: newDisplayName}})
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

		if !words.IsValidWord(answer) {
			lobby.logger.Printf("%s submitted %s for challenge %s - rejected because it's not a word",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: ANSWER_REJECTED, Content: answer})
			return
		}

		if answer == lobby.currentChallenge {
			lobby.logger.Printf("%s submitted %s for challenge %s - rejected because it's the same as the challenge",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: ANSWER_REJECTED, Content: answer})
			return
		}

		if !strings.Contains(answer, lobby.currentChallenge) {
			lobby.logger.Printf("%s submitted %s for challenge %s - rejected because it does not contain the challenge",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: ANSWER_REJECTED, Content: answer})
			return
		}

		lobby.logger.Printf("%s submitted %s for challenge %s - accepted", lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
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
		newTurnIndex := (lobby.turnIndex + 1) % len(lobby.aliveClients)
		if lobby.turnIndex > -1 {
			lobby.logger.Printf("Changing turn from %s to %s", lobby.aliveClients[lobby.turnIndex], lobby.aliveClients[newTurnIndex])
		} else {
			lobby.logger.Printf("Starting turn with %s", lobby.aliveClients[newTurnIndex])
		}
		lobby.turnIndex = newTurnIndex
	} else {
		eliminatedClient := lobby.aliveClients[lobby.turnIndex]
		// if they ran out of time or disconnected:
		// - kick them out of the aliveClients
		// - turnIndex can stay the same (since the next client will now occupy that index)
		//   unless the last client got eliminated, in which case just need to reset the turnIndex to 0
		aliveClients := make([]*Client, 0, len(lobby.aliveClients)-1)
		for _, c := range lobby.aliveClients {
			if c.Id != eliminatedClient.Id {
				aliveClients = append(aliveClients, c)
			}
		}
		lobby.aliveClients = aliveClients

		if lobby.turnIndex == len(lobby.aliveClients) {
			lobby.turnIndex = 0
		}

		lobby.logger.Printf("Changing turn from %s (eliminated) to %s", eliminatedClient, lobby.aliveClients[lobby.turnIndex])
	}

	lobby.currentChallenge = words.GetChallenge()
	lobby.BroadcastMessage(Message{
		Type: CLIENTS_TURN,
		Content: ClientsTurnContent{
			ClientId:  lobby.aliveClients[lobby.turnIndex].Id,
			Challenge: lobby.currentChallenge,
			Time:      TURN_LIMIT_SECONDS,
		},
	})
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
