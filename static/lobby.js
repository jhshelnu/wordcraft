const START_GAME         = "start_game"         // the game has started
const CLIENT_ID_ASSIGNED = "client_id_assigned" // sent to a newly connected player, indicating their id
const CLIENT_JOINED      = "client_joined"      // a new client has joined
const CLIENT_LEFT        = "client_left"        // a client has left
const CURRENT_ANSWER     = "current_answer"     // for the current turn, what is currently the answer
const ANSWER_ACCEPTED    = "answer_accepted"    // the answer is accepted
const ANSWER_REJECTED    = "answer_rejected"    // the answer is not accepted, the player has lost
const CLIENTS_TURN       = "clients_turn"       // it's a new clients turn

document.addEventListener("DOMContentLoaded", () => {
    let ws = new WebSocket(`ws://localhost:8080/ws/${lobbyId}`)

    let sendButton = document.getElementById("send-button")
    let input = document.getElementById("send-text")

    let clientId; // our assigned id for the lobby we're joining

    ws.onmessage = ({ data }) => {
        let message = JSON.parse(data)
        switch (message["Type"]) {
            case CLIENT_ID_ASSIGNED:
                clientId = parseInt(message["Arg"])
                alert(`I am player ${clientId}!`)
                break;
            case CLIENT_JOINED:
                let newClientId = parseInt(message["Arg"])
                if (newClientId !== clientId) {
                    alert(`Player ${newClientId} has joined!`)
                }
                break;
            case CLIENT_LEFT:
                let leftClientId = parseInt(message["Arg"])
                alert(`Player ${leftClientId} has left`)
                break;
        }
    }

    sendButton.addEventListener("click", () => {
        if (ws.readyState !== WebSocket.OPEN) {
            return
        }
        // ws.send(JSON.stringify({ content: input.value }))
    })
})