package game

import "fmt"

type messageType string

//goland:noinspection GoNameStartsWithPackageName
const (
	StartGame        messageType = "start_game"         // the game has started
	ClientDetails    messageType = "client_details"     // sent to a newly connected client, indicating their id, the status of the game, etc
	ClientDetailsReq messageType = "client_details_req" // sent from a client that has reconnected asking the server for a full update
	ClientJoined     messageType = "client_joined"      // a new client has joined
	ClientLeft       messageType = "client_left"        // a client has left
	SubmitAnswer     messageType = "submit_answer"      // when the client submits an answer
	AnswerPreview    messageType = "answer_preview"     // preview of the current answer (not submitted) so other clients can see
	AnswerAccepted   messageType = "answer_accepted"    // the answer is accepted
	AnswerRejected   messageType = "answer_rejected"    // the answer is not accepted
	TurnExpired      messageType = "turn_expired"       // client has run out of time
	ClientsTurn      messageType = "clients_turn"       // it's a new clients turn
	GameOver         messageType = "game_over"          // the game is over
	RestartGame      messageType = "restart_game"       // sent from a client to initiate a game restart. sever then rebroadcasts to all clients to confirm
	NameChange       messageType = "name_change"        // used by clients to indicate they want a new display name
	Shutdown         messageType = "shutdown"           // tells the clients the server is being shutdown now
)

type Message struct {
	From    int         // id of the Client in the lobby
	Type    messageType // content of the message
	Content any         // any additional info, e.g. which client joined, what their answer is, etc
}

func (m Message) String() string {
	return fmt.Sprintf("Message[Type='%s']", m.Type)
}

type ClientsTurnContent struct {
	ClientId  int    // whose turn it is
	Challenge string // what the challenge string is, e.g. "atr"
	TurnEnd   int64  // milliseconds from unix epoch (UTC)
	Now       int64  // current time according to the server
}

type TurnExpiredContent struct {
	EliminatedClientId int      // id of the client who just went out
	Suggestions        []string // some common words they could have answered with
}

// ClientDetailsContent is broadcast from the server to one particular client at the moment of connection
// it's job is to catch the client up on details-- what their id is, the current state of the game, etc
type ClientDetailsContent struct {
	ClientId          int             // the id assigned to this client
	ReconnectToken    string          // a token used to reconnect to the lobby as an existing player (during browser refresh/temp connection loss)
	Status            gameStatus      // the status of the game (if a client connects mid-game or when the game is over, this is how they'll know)
	Clients           []ClientContent // details of the existing clients in the lobby
	CurrentTurnId     int             // the id of the client whose turn it is (or 0 if not applicable)
	CurrentChallenge  string          // what the current challenge is, or "" if there isn't one
	CurrentAnswerPrev string          // what the client whose turn it is currently has typed in
	TurnEnd           int64           // milliseconds from unix epoch (UTC), or 0 if not applicable
	Now               int64           // current server time
	WinnersName       string          // name of the client who won (at the moment of winning), or "" if not applicable
}

// ClientJoinedContent is broadcast to all clients when a new client joins
type ClientJoinedContent struct {
	ClientId    int    // the id of the newly joined client
	DisplayName string // what their name is
	IconName    string // which icon they are using
	Alive       bool   // whether they are alive or not
}

type ClientNameChangeContent struct {
	ClientId       int    // who is changing their name
	NewDisplayName string // what they are changing their name to
}

// ClientContent is not currently sent as a standalone message content, but embedded
// within ClientDetailsContent. It represents the current state of another client in the lobby
type ClientContent struct {
	Id          int
	DisplayName string
	IconName    string
	Alive       bool
}
