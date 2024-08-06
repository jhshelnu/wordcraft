// message types
const START_GAME         = "start_game"         // the game has started
const CLIENT_ID_ASSIGNED = "client_id_assigned" // sent to a newly connected client, indicating their id
const CLIENT_JOINED      = "client_joined"      // a new client has joined
const CLIENT_LEFT        = "client_left"        // a client has left
const SUBMIT_ANSWER      = "submit_answer"      // when the client submits an answer
const ANSWER_PREVIEW     = "answer_preview"     // preview of the current answer (not submitted) so other clients can see
const ANSWER_ACCEPTED    = "answer_accepted"    // the answer is accepted
const ANSWER_REJECTED    = "answer_rejected"    // the answer is not accepted
const TURN_EXPIRED       = "turn_expired"       // client has run out of time
const CLIENTS_TURN       = "clients_turn"       // it's a new clients turn
const GAME_OVER          = "game_over"          // the game is over

let clientsTurnId;        // the id of the client whose turn it is
let clientId              // our assigned id for the lobby we're joining
let startGameButton       // the button to start the game

let challengeInputSection // the part of the page to get the user's input (only shown during their turn)
let challengeText         // the text displaying the current challenge to the user
let answerInput           // the input element which holds what the user has typed so far
let statusText            // large text at the top of the screen displaying the current challenge

document.addEventListener("DOMContentLoaded", () => {
    // establish websocket connection right away
    let ws = new WebSocket(`ws://localhost:8080/ws/${lobbyId}`)

    startGameButton = document.getElementById("start-game-button")
    challengeInputSection = document.getElementById("challenge-input-section")
    answerInput = document.getElementById("answer-input")
    challengeText = document.getElementById("challenge-text")
    statusText = document.getElementById("status-text")

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
                onAnswerPreview(content)
                break
            case ANSWER_ACCEPTED:
                break
            case ANSWER_REJECTED:
                onAnswerRejected()
                break
            case GAME_OVER:
                onGameOver()
                break
        }
    }

    answerInput.addEventListener("input", () => {
        let currentInput = answerInput.value
        ws.send(JSON.stringify({ Type: ANSWER_PREVIEW, Content: currentInput }))
    })

    answerInput.addEventListener('keyup', e => {
        let input = answerInput.value
        if (e.key === 'Enter' && input) {
            ws.send(JSON.stringify({ Type: SUBMIT_ANSWER, Content: input }))
        }
    })
})

function onClientJoined(newClientId) {
    renderNewClientCard(newClientId)
    if (document.getElementById("clients-list").children.length >= 2) {
        startGameButton.textContent = "Start game!"
        startGameButton.removeAttribute("disabled")
    }
}

function onClientLeft(leavingClientId) {
    document.querySelector(`#clients-list [data-client-id="${leavingClientId}"]`).remove()
}

function onClientsTurn(content) {
    startGameButton.style.display = "none"

    let newClientsTurnId = content["ClientId"]
    challengeText.textContent = content["Challenge"]
    statusText.style.display = "block"

    if (clientsTurnId) {
        document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`).style.visibility = "hidden"
    }

    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess]`).textContent = ""
    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess-pill]`).style.visibility = "visible"

    if (clientId === newClientsTurnId) {
        // it's our turn
        answerInput.value = ""
        challengeInputSection.style.display = "block"
    } else {
        // it's not our turn
        challengeInputSection.style.display = "none"
    }

    clientsTurnId = newClientsTurnId
}

function onAnswerPreview(answerPreview) {
    let answerPreviewText = answerPreview.length <= 20 ? answerPreview : answerPreview.substring(0, 20).concat("...")
    document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess]`).textContent = answerPreviewText
}

function onAnswerRejected() {
    if (clientsTurnId === clientId) {
        shakeElement(answerInput, 20)
    }

    let pill = document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`)
    shakeElement(pill, 10)
}

function shakeElement(e, amt) {
    gsap.to(e, {
        x: -amt,
        duration: 0.03,
        ease: Sine.easeIn,
        onComplete: () => {
            gsap.fromTo(e, { x: -amt }, {
                x: amt,
                repeat: 2,
                duration: 0.06,
                yoyo: true,
                ease: Sine.easeInOut,
                onComplete: () => {
                    gsap.to(e, {
                        x: 0,
                        ease: Elastic.easeOut
                    })
                }
            })
        }
    })
}

function onGameOver() {
    // todo: announce the end of the game better
    console.log('the game is over')
}