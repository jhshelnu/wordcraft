// message types
const START_GAME         = "start_game"         // the game has started
const CLIENT_ID_ASSIGNED = "client_id_assigned" // sent to a newly connected client, indicating their id
const CLIENT_JOINED      = "client_joined"      // a new client has joined
const CLIENT_LEFT        = "client_left"        // a client has left
const SUBMIT_ANSWER      = "submit_answer"      // when the client submits an answer
const ANSWER_PREVIEW     = "answer_preview"     // preview of the current answer (not submitted) so other clients can see
const ANSWER_ACCEPTED    = "answer_accepted"    // the answer is accepted
const ANSWER_REJECTED    = "answer_rejected"    // the answer is not accepted
const TIMES_UP           = "times_up"           // client has run out of time
const CLIENTS_TURN       = "clients_turn"       // it's a new clients turn

// game statuses
const WAITING_FOR_PLAYERS = 0
const IN_PROGRESS         = 1
const GAME_OVER           = 2

let clientId;              // our assigned id for the lobby we're joining
let clientsList;           // the <ul> element holding the list of players that are currently connected
let startGameButton;
let challengeInputSection; // the part of the page to get the user's input (only shown during their turn)
let answerInput;
let submitAnswerButton;

document.addEventListener("DOMContentLoaded", () => {
    // establish websocket connection right away
    let ws = new WebSocket(`ws://localhost:8080/ws/${lobbyId}`)

    clientsList = document.getElementById("clients-list")
    startGameButton = document.getElementById("start-game-button")
    challengeInputSection = document.getElementById("challenge-input-section")
    answerInput = document.getElementById("answer-input")
    submitAnswerButton = document.getElementById("submit-answer")

    if (gameStatus === WAITING_FOR_PLAYERS) {
        startGameButton.style.display = "inline"

    }

    startGameButton.addEventListener("click", () => {
        ws.send(JSON.stringify({ Type: START_GAME }))
    })

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
            case ANSWER_PREVIEW:
                console.log(`player submitted: ${content}`)
                break
            case ANSWER_ACCEPTED:
                console.log(`answer ${content} accepted`)
                break
            case ANSWER_REJECTED:
                console.log(`answer ${content} rejected`)
                break
        }
    }

    answerInput.addEventListener("change", () => {
        let currentInput = answerInput.value
        ws.send(JSON.stringify({ Type: ANSWER_PREVIEW, Content: currentInput }))
    })


    submitAnswerButton.addEventListener("click", () => {
        let input = answerInput.value
        if (ws.readyState !== WebSocket.OPEN || !input) {
            return
        }
        ws.send(JSON.stringify({ Type: SUBMIT_ANSWER, Content: input }))
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



    // todo: un-hide the challenge input html, transmit answer
}