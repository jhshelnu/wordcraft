package game

import (
	"fmt"
	"log"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jhshelnu/wordcraft/icons"
	"github.com/jhshelnu/wordcraft/words"
)

const (
	MaxDisplayName = 15
)

//go:generate go run golang.org/x/tools/cmd/stringer -type gameStatus
type gameStatus int

const (
	WaitingForPlayers gameStatus = iota
	InProgress
	Over
)

type Lobby struct {
	Id string // the unique identifier for this lobby

	logger *log.Logger

	join  chan *Client // channel for new clients to join the lobby
	leave chan *Client // channel for existing clients to leave the lobby
	read  chan Message // channel for existing clients to send messages for the Lobby to read

	iconNames []string // a slice of icon file names (shuffled for each lobby)

	// todo: consider refactoring these fields into a game state struct for better code separation
	clients           map[int]*Client  // all clients in the lobby, indexed by their id
	aliveClients      []*Client        // all clients in the lobby who are not out
	status            gameStatus       // the status of the game, indicates if its started, in progress, etc
	turnIndex         int              // the index in aliveClients of whose turn it is
	turnRounds        int              // how many times the turn has changed to the first player (lowest client id)
	currentChallenge  string           // the current challenge string for clientsTurn
	currentAnswerPrev string           // preview of what the client whose turn it is has typed so far
	currentTurnEnd    int64            // when the current turn ends, in milliseconds from the unix epoch (UTC)
	turnExpired       <-chan time.Time // a (read-only) channel which produces a single boolean value once the client has run out of time
	winnersName       string           // the name of the winning client (captured at the moment they won) this is for new clients joining after the game

	lastClientId  int        // the id of the last client which connected (used to increment Client.id's as they join the lobby)
	clientIdMutex sync.Mutex // enforces thread-safe access to the nextClientId

	lobbyEndChan chan string // channel that lets this lobby notify the main thread that this lobby has completed. This allows the Lobby to get GC'ed
}

func NewLobby(id string, lobbyEndChan chan string) *Lobby {
	logger := log.New(os.Stdout, fmt.Sprintf("Lobby [%s]: ", id), log.Lshortfile|log.Lmsgprefix)
	return &Lobby{
		logger:       logger,
		Id:           id,
		join:         make(chan *Client),
		leave:        make(chan *Client),
		read:         make(chan Message),
		iconNames:    icons.GetShuffledIconNames(),
		status:       WaitingForPlayers,
		clients:      make(map[int]*Client),
		turnIndex:    -1,
		lobbyEndChan: lobbyEndChan,
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

func (lobby *Lobby) GetClientCount() int {
	return len(lobby.clients)
}

func (lobby *Lobby) GetClientByReconnectToken(reconnectToken string) *Client {
	if reconnectToken == "" {
		return nil
	}

	for _, c := range lobby.clients {
		if c.reconnectToken == reconnectToken {
			return c
		}
	}

	return nil
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

func (lobby *Lobby) BroadcastShutdown() {
	lobby.BroadcastMessage(Message{Type: Shutdown})
}

func (lobby *Lobby) onClientJoin(joiningClient *Client) {
	lobby.logger.Printf("%s connected", joiningClient)

	// tell all the existing clients about the joiningClient
	lobby.BroadcastMessage(Message{Type: ClientJoined, Content: ClientJoinedContent{
		ClientId:    joiningClient.id,
		DisplayName: joiningClient.displayName,
		IconName:    joiningClient.iconName,
		// for new clients, they are considered alive if they join mid-game or after the game
		Alive: lobby.status != InProgress,
	}})

	lobby.clients[joiningClient.id] = joiningClient
	if lobby.status != InProgress {
		lobby.aliveClients = append(lobby.aliveClients, joiningClient)
	}

	// then tell the joiningClient about the entire state of the game
	joiningClient.write <- Message{Type: ClientDetails, Content: lobby.buildClientDetails(joiningClient)}
}

func (lobby *Lobby) onClientLeave(leavingClient *Client) {
	// clients are really two goroutines (for reading and writing) which will both announce their exit to the server
	// so, need to prevent firing duplicate messages when they leave
	if _, exists := lobby.clients[leavingClient.id]; !exists {
		return
	}

	lobby.logger.Printf("%s disconnected", leavingClient)

	delete(lobby.clients, leavingClient.id)
	lobby.BroadcastMessage(Message{Type: ClientLeft, Content: leavingClient.id})

	// the rest of the code in here is concerned with leaving aliveClients in a consistent state (and declaring a winner if necessary)
	// if the leaving client is already eliminated, then there is nothing left to do
	if !slices.Contains(lobby.aliveClients, leavingClient) {
		return
	}

	// if they are alive but the game is not currently in progress, just need to evict them from the aliveClients and call it a day
	if lobby.status != InProgress {
		lobby.aliveClients = slices.DeleteFunc(lobby.aliveClients, func(c *Client) bool { return c == leavingClient })
		return
	}

	// but, if the game is in progress, then potentially a winner needs to be declared
	if len(lobby.aliveClients) == 2 {
		lobby.aliveClients = slices.DeleteFunc(lobby.aliveClients, func(c *Client) bool { return c == leavingClient })
		lobby.endGame()
		return
	}

	// ok, client was alive, game was in progress, and there are enough players to continue the game.

	// if a client leaves during their turn, change the turn to the next client
	if lobby.aliveClients[lobby.turnIndex] == leavingClient {
		lobby.logger.Printf("Changing the current turn because %s left while it was their turn", leavingClient)
		lobby.changeTurn(true)
		return
	}

	// if it's not their turn, remove them from aliveClients, but may need to adjust the turnIndex
	// note that we don't call changeTurn since we aren't actually changing the turn to another player (and dont want to broadcast a change turn message)
	leavingClientTurnIndex := slices.Index(lobby.aliveClients, leavingClient)
	lobby.aliveClients = slices.DeleteFunc(lobby.aliveClients, func(c *Client) bool { return c == leavingClient })
	if leavingClientTurnIndex < lobby.turnIndex {
		// ensure turnIndex stays pointed at the same client
		lobby.turnIndex--
	}
}

func (lobby *Lobby) onMessage(message Message) {
	switch message.Type {
	case StartGame:
		lobby.onStartGame(message)
	case RestartGame:
		lobby.onRestartGame(message)
	case AnswerPreview:
		lobby.onAnswerPreview(message)
	case SubmitAnswer:
		lobby.onAnswerSubmitted(message)
	case NameChange:
		lobby.onNameChange(message)
	case ClientDetailsReq:
		lobby.onClientDetailsReq(message)
	default:
		lobby.logger.Printf("Received message with type %s. Ignoring due to no handler function", message.Type)
	}
}

func (lobby *Lobby) onTurnExpired() {
	// sometimes, depending on timing, our timer can fire after the players have left
	if lobby.status != InProgress {
		return
	}

	eliminatedClient := lobby.aliveClients[lobby.turnIndex]
	lobby.BroadcastMessage(Message{Type: TurnExpired, Content: TurnExpiredContent{
		EliminatedClientId: eliminatedClient.id,
		Suggestions:        words.GetChallengeSuggestions(lobby.currentChallenge),
	}})

	lobby.changeTurn(true)
}

func (lobby *Lobby) onStartGame(message Message) {
	if lobby.status == WaitingForPlayers && len(lobby.clients) >= 2 {
		lobby.logger.Printf("%s has started the game", lobby.clients[message.From])
		lobby.status = InProgress
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) onRestartGame(message Message) {
	if lobby.status == Over && len(lobby.clients) >= 2 {
		lobby.logger.Printf("%s has restarted the game", lobby.clients[message.From])
		lobby.resetAliveClients()
		lobby.status = InProgress
		lobby.turnIndex = -1
		lobby.turnRounds = 0
		lobby.BroadcastMessage(Message{Type: RestartGame})
		lobby.changeTurn(false)
	}
}

func (lobby *Lobby) resetAliveClients() {
	// reset alive clients to hold all clients
	lobby.aliveClients = slices.SortedFunc(maps.Values(lobby.clients), func(c1 *Client, c2 *Client) int {
		return c1.id - c2.id
	})
}

func (lobby *Lobby) onNameChange(message Message) {
	newDisplayName, ok := message.Content.(string)
	if !ok || len(newDisplayName) > MaxDisplayName {
		return
	}

	client := lobby.clients[message.From]
	client.displayName = newDisplayName
	lobby.BroadcastMessage(Message{Type: NameChange, Content: ClientNameChangeContent{ClientId: client.id, NewDisplayName: newDisplayName}})
}

func (lobby *Lobby) onClientDetailsReq(message Message) {
	client := lobby.clients[message.From]
	clientDetailsContent := lobby.buildClientDetails(client)
	client.write <- Message{Type: ClientDetails, Content: clientDetailsContent}
}

func (lobby *Lobby) onAnswerPreview(message Message) {
	if lobby.status == InProgress && message.From == lobby.aliveClients[lobby.turnIndex].id {
		currentAnswerPrev, ok := message.Content.(string)
		if ok {
			lobby.currentAnswerPrev = currentAnswerPrev
			lobby.BroadcastMessage(Message{Type: AnswerPreview, Content: lobby.currentAnswerPrev})
		}
	}
}

func (lobby *Lobby) onAnswerSubmitted(message Message) {
	if lobby.status == InProgress && message.From == lobby.aliveClients[lobby.turnIndex].id {
		answer, ok := message.Content.(string)
		if !ok {
			return
		}

		if !words.IsValidWord(answer) {
			lobby.logger.Printf("%s submitted '%s' for challenge '%s' - rejected because it's not a word",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: AnswerRejected, Content: answer})
			return
		}

		if answer == lobby.currentChallenge {
			lobby.logger.Printf("%s submitted %s for challenge %s - rejected because it's the same as the challenge",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: AnswerRejected, Content: answer})
			return
		}

		if !strings.Contains(answer, lobby.currentChallenge) {
			lobby.logger.Printf("%s submitted %s for challenge %s - rejected because it does not contain the challenge",
				lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
			lobby.BroadcastMessage(Message{Type: AnswerRejected, Content: answer})
			return
		}

		lobby.logger.Printf("%s submitted %s for challenge %s - accepted", lobby.aliveClients[lobby.turnIndex], answer, lobby.currentChallenge)
		lobby.BroadcastMessage(Message{Type: AnswerAccepted, Content: answer})
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
		lobby.aliveClients = slices.DeleteFunc(lobby.aliveClients, func(c *Client) bool { return c == eliminatedClient })
		if len(lobby.aliveClients) == 1 {
			lobby.endGame()
			return
		}

		// this happens if the last aliveClient in the list leaves
		if lobby.turnIndex == len(lobby.aliveClients) {
			lobby.turnIndex = 0
		}
		lobby.logger.Printf("Changing turn from %s (eliminated) to %s", eliminatedClient, lobby.aliveClients[lobby.turnIndex])
	}

	if lobby.turnIndex == 0 {
		lobby.turnRounds++
	}

	turnLimitDuration := lobby.getTurnLimitDuration()
	lobby.currentTurnEnd = time.Now().Add(turnLimitDuration).UnixMilli()
	lobby.turnExpired = time.After(turnLimitDuration)
	lobby.currentChallenge = words.GetChallenge(lobby.getTurnDifficulty())

	lobby.BroadcastMessage(Message{
		Type: ClientsTurn,
		Content: ClientsTurnContent{
			ClientId:  lobby.aliveClients[lobby.turnIndex].id,
			Challenge: lobby.currentChallenge,
			TurnEnd:   lobby.currentTurnEnd,
			Now:       time.Now().UnixMilli(),
		},
	})
}

// assumes that lobby.aliveClients == 1 and the winner is lobby.aliveClients[0]
func (lobby *Lobby) endGame() {
	lobby.status = Over
	lobby.winnersName = lobby.aliveClients[0].displayName
	lobby.BroadcastMessage(Message{Type: GameOver, Content: lobby.aliveClients[0].id})
}

func (lobby *Lobby) getTurnDifficulty() words.ChallengeDifficulty {
	if lobby.turnRounds > 10 {
		return words.ChallengeHard
	} else if lobby.turnRounds > 4 {
		return words.ChallengeMedium
	} else {
		return words.ChallengeEasy
	}
}

func (lobby *Lobby) getTurnLimitDuration() time.Duration {
	switch true {
	case lobby.turnRounds > 12:
		return 16 * time.Second // rounds 13+: 16 seconds
	case lobby.turnRounds > 5:
		return 18 * time.Second // rounds 6-12: 18 seconds
	case lobby.turnRounds > 1:
		return 20 * time.Second // rounds 2-5: 20 seconds
	case lobby.turnRounds == 1:
		return 25 * time.Second // round 1: 25 seconds (give them bonus time to get familiar with the game)
	default:
		lobby.logger.Printf("WARN: No turnLimit duration specified for %d turnRounds. Falling back to 20 second default.", lobby.turnRounds)
		return 20 * time.Second
	}
}

// buildClientDetails is responsible for building and returning a ClientDetailsContent struct
// which contains the current state of the lobby for a newly connected (or reconnected) client, so they can get caught up
func (lobby *Lobby) buildClientDetails(client *Client) ClientDetailsContent {
	isAliveMap := make(map[*Client]bool, len(lobby.aliveClients))
	for _, c := range lobby.aliveClients {
		isAliveMap[c] = true
	}

	// sorted slice of clients (ensures ordering of clients is consistent for all players)
	clients := slices.SortedFunc(maps.Values(lobby.clients), func(c1, c2 *Client) int {
		return c1.id - c2.id
	})
	clientContents := make([]ClientContent, 0, len(lobby.clients))
	for _, c := range clients {
		clientContents = append(clientContents, ClientContent{
			Id:          c.id,
			DisplayName: c.displayName,
			IconName:    c.iconName,
			Alive:       isAliveMap[c],
		})
	}

	var currentTurnId int
	if lobby.status == InProgress {
		currentTurnId = lobby.aliveClients[lobby.turnIndex].id
	} else {
		currentTurnId = 0
	}

	return ClientDetailsContent{
		ClientId:          client.id,
		ReconnectToken:    client.reconnectToken,
		Status:            lobby.status,
		Clients:           clientContents,
		CurrentTurnId:     currentTurnId,
		CurrentChallenge:  lobby.currentChallenge,
		CurrentAnswerPrev: lobby.currentAnswerPrev,
		TurnEnd:           lobby.currentTurnEnd,
		Now:               time.Now().UnixMilli(),
		WinnersName:       lobby.winnersName,
	}
}

func (lobby *Lobby) BroadcastMessage(message Message) {
	for _, c := range lobby.clients {
		c.write <- message
	}
}

func (lobby *Lobby) EndLobby() {
	lobby.lobbyEndChan <- lobby.Id
}
