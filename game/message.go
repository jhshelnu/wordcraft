package game

type MessageType string

const (
	START_GAME             = "start_game"             // the game has started
	CLIENT_ID_ASSIGNED     = "client_id_assigned"     // sent to a newly connected player, indicating their id
	CLIENT_JOINED          = "client_joined"          // a new client has joined
	CLIENT_LEFT            = "client_left"            // a client has left
	CURRENT_ANSWER         = "current_answer"         // for the current turn, what is currently the answer
	CURRENT_ANSWER_PREVIEW = "current_answer_preview" // preview of the current answer (not submitted) so other clients can see
	ANSWER_ACCEPTED        = "answer_accepted"        // the answer is accepted
	ANSWER_REJECTED        = "answer_rejected"        // the answer is not accepted, the player has lost
	CLIENTS_TURN           = "clients_turn"           // it's a new clients turn
)

type Message struct {
	From    int         // id of the Client in the lobby
	Type    MessageType // content of the message
	Content any         // any additional info, e.g. which client joined, what their answer is, etc
}

type ClientsTurnContent struct {
	ClientId  int    // whose turn it is
	Challenge string // what the challenge string is, e.g. "atr"
}
