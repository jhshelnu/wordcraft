package game

type messageType string

const (
	START_GAME      messageType = "start_game"      // the game has started
	CLIENT_DETAILS              = "client_details"  // sent to a newly connected client, indicating their id, the status of the game, etc
	CLIENT_JOINED               = "client_joined"   // a new client has joined
	CLIENT_LEFT                 = "client_left"     // a client has left
	SUBMIT_ANSWER               = "submit_answer"   // when the client submits an answer
	ANSWER_PREVIEW              = "answer_preview"  // preview of the current answer (not submitted) so other clients can see
	ANSWER_ACCEPTED             = "answer_accepted" // the answer is accepted
	ANSWER_REJECTED             = "answer_rejected" // the answer is not accepted
	TURN_EXPIRED                = "turn_expired"    // client has run out of time
	CLIENTS_TURN                = "clients_turn"    // it's a new clients turn
	GAME_OVER                   = "game_over"       // the game is over
	RESTART_GAME                = "restart_game"    // sent from a client to initiate a game restart. sever then rebroadcasts to all clients to confirm
	NAME_CHANGE                 = "name_change"     // used by clients to indicate they want a new display name
)

type Message struct {
	From    int         // id of the Client in the lobby
	Type    messageType // content of the message
	Content any         // any additional info, e.g. which client joined, what their answer is, etc
}

type ClientsTurnContent struct {
	ClientId  int    // whose turn it is
	Challenge string // what the challenge string is, e.g. "atr"
	Time      int    // how many seconds the user has to submit a valid answer
}

// ClientDetailsContent is broadcast from the server to one particular client at the moment of connection
// it's job is to catch the client up on details-- what their id is, the current state of the game, etc
type ClientDetailsContent struct {
	ClientId int // the id assigned to this client
}

// ClientJoinedContent is broadcast to all clients when a new client joins
type ClientJoinedContent struct {
	ClientId    int    // the id of the newly joined client
	DisplayName string // what their name is
	IconName    string // which icon they are using
}

type ClientNameChange struct {
	ClientId       int    // who is changing their name
	NewDisplayName string // what they are changing their name to
}
