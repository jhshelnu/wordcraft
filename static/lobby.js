// message types
const START_GAME         = "start_game"         // the game has started
const CLIENT_ID_ASSIGNED = "client_id_assigned" // sent to a newly connected player, indicating their id
const CLIENT_JOINED      = "client_joined"      // a new client has joined
const CLIENT_LEFT        = "client_left"        // a client has left
const CURRENT_ANSWER     = "current_answer"     // for the current turn, what is currently the answer
const ANSWER_ACCEPTED    = "answer_accepted"    // the answer is accepted
const ANSWER_REJECTED    = "answer_rejected"    // the answer is not accepted, the player has lost
const CLIENTS_TURN       = "clients_turn"       // it's a new clients turn

// game statuses
const WAITING_FOR_PLAYERS = 0
const IN_PROGRESS         = 1
const GAME_OVER           = 2

let clientId;    // our assigned id for the lobby we're joining
let clientsList; // the <ul> element holding the list of players that are currently connected
let startGameButton;

document.addEventListener("DOMContentLoaded", () => {
    clientsList = document.getElementById("clients-list")

    startGameButton = document.getElementById("start-game-button")
    if (gameStatus === WAITING_FOR_PLAYERS) {
        startGameButton.style.display = "inline"
    }
    startGameButton.addEventListener("click", () => {
        ws.send(JSON.stringify({ Type: START_GAME }))
    })

    let sendButton = document.getElementById("send-button")
    let input = document.getElementById("send-text")

    let ws = new WebSocket(`ws://localhost:8080/ws/${lobbyId}`)
    ws.onmessage = ({ data }) => {
        let message = JSON.parse(data)
        let type = message["Type"]
        let content = message["Content"]
        switch (type) {
            case CLIENT_ID_ASSIGNED:
                clientId = content
                break
            case CLIENT_JOINED:
                onClientJoined(content)
                break
            case CLIENT_LEFT:
                onClientLeft(content)
                break
            case CLIENTS_TURN:
                onClientsTurn(content)
                break
        }
    }

    sendButton.addEventListener("click", () => {
        if (ws.readyState !== WebSocket.OPEN) {
            return
        }
        // ws.send(JSON.stringify({ content: input.value }))
    })
})

function onClientJoined(newClientId) {
    let elem = document.createElement("li")
    elem.setAttribute("data-client-id", newClientId)
    elem.append(document.createTextNode(newClientId))
    clientsList.append(elem)
}

function onClientLeft(leavingClientId) {
    document.querySelector(`#clients-list li[data-client-id="${leavingClientId}"]`).remove()
}

function onClientsTurn(content) {
    gameStatus = IN_PROGRESS
    startGameButton.style.display = "none"

    let clientsTurn = content["ClientId"]
    let challenge = content["Challenge"]
    alert(`It is ${clientsTurn === clientId ? "my turn" : "not my turn"}. Challenge is: ${challenge}`)

    // todo: un-hide the challenge input html, transmit answer
}