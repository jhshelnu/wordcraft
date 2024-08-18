// message types
const START_GAME     = "start_game"      // the game has started
const CLIENT_DETAILS = "client_details"    // sent to a newly connected client, indicating their id
const CLIENT_JOINED  = "client_joined"   // a new client has joined
const CLIENT_LEFT    = "client_left"     // a client has left
const SUBMIT_ANSWER  = "submit_answer"   // when the client submits an answer
const ANSWER_PREVIEW = "answer_preview"  // preview of the current answer (not submitted) so other clients can see
const ANSWER_ACCEPTED= "answer_accepted" // the answer is accepted
const ANSWER_REJECTED= "answer_rejected" // the answer is not accepted
const TURN_EXPIRED   = "turn_expired"    // client has run out of time
const CLIENTS_TURN   = "clients_turn"    // it's a new clients turn
const GAME_OVER      = "game_over"       // the game is over
const RESTART_GAME   = "restart_game"    // sent from a client to initiate a game restart. sever then rebroadcasts to all clients to confirm
const NAME_CHANGE    = "name_change"     // used by clients to indicate they want a new display name

let ws                    // the websocket connection
let clientId              // our assigned id for the lobby we're joining
let myDisplayNameInput    // the <input> which holds our current displayName
let startGameButton       // the button to start the game
let restartGameButton     // the button to restart the game
let inviteButton          // the button that copies the lobby link to the clipboard
let inviteButtonText      // the text of the invite button (changes after being clicked)
let clientsTurnId         // the id of the client whose turn it is
let challengeInputSection // the part of the page to get the user's input (only shown during their turn)
let answerInput           // the input element which holds what the user has typed so far
let statusText            // large text at the top of the screen displaying the current status (current challenge, who won, etc)
let turnCountdownInterval // the interval where we count down how many seconds the user has left

const VOLUME = 0.4 // how loud to play the audio
let answerAcceptedAudio    // what plays when an answer is accepted
let clientJoinedAudio      // what plays when another client joins
let clientEliminated       // what plays when time runs out for a client

document.addEventListener("DOMContentLoaded", () => {
    // establish websocket connection right away
    ws = new WebSocket(`ws://${location.host}/ws/${lobbyId}`)
    startGameButton = document.getElementById("start-game-button")
    restartGameButton = document.getElementById("restart-game-button")
    inviteButton = document.getElementById("invite-button")
    inviteButtonText = document.getElementById("invite-button-text")
    challengeInputSection = document.getElementById("challenge-input-section")
    answerInput = document.getElementById("answer-input")
    statusText = document.getElementById("status-text")

    answerAcceptedAudio = new Audio("/static/sounds/answer_accepted.mp3")
    clientJoinedAudio   = new Audio("/static/sounds/client_joined.mp3")
    clientEliminated    = new Audio("/static/sounds/client_eliminated.wav")

    startGameButton.addEventListener("click", () => {
        ws.send(JSON.stringify({ Type: START_GAME }))
    })

    restartGameButton.addEventListener("click", () => {
        ws.send(JSON.stringify({ Type: RESTART_GAME }))
    })

    inviteButton.addEventListener("click", async () => {
        await navigator.clipboard.writeText(location.href)
        inviteButtonText.textContent = "Copied!"
    })

    ws.onmessage = ({ data }) => {
        let message = JSON.parse(data)
        let type = message["Type"]
        let content = message["Content"]
        switch (type) {
            case CLIENT_DETAILS:
                onClientDetails(content)
                break
            case CLIENT_JOINED:
                onClientJoined(content)
                break
            case CLIENT_LEFT:
                onClientLeft(content)
                break
            case NAME_CHANGE:
                onNameChange(content)
                break
            case CLIENTS_TURN:
                onClientsTurn(content)
                break
            case ANSWER_PREVIEW:
                onAnswerPreview(content)
                break
            case ANSWER_ACCEPTED:
                onAnswerAccepted()
                break
            case ANSWER_REJECTED:
                onAnswerRejected()
                break
            case TURN_EXPIRED:
                onTurnExpired(content)
                break
            case GAME_OVER:
                onGameOver(content)
                break
            case RESTART_GAME:
                onRestartGame()
                break
        }
    }

    answerInput.addEventListener("input", () => {
        let currentInput = answerInput.value.toLowerCase()
        ws.send(JSON.stringify({ Type: ANSWER_PREVIEW, Content: currentInput }))
    })

    answerInput.addEventListener("keyup", e => {
        let input = answerInput.value.toLowerCase()
        if (e.key === "Enter" && input) {
            ws.send(JSON.stringify({ Type: SUBMIT_ANSWER, Content: input }))
        }
    })
})

// this message is broadcast from the server to one particular client at the moment of connection
// it's job is to catch the client up on details-- what their id is, the current state of the game, etc
function onClientDetails(content) {
    clientId = content["ClientId"]
}

function onClientJoined(content) {
    let newClientId = content["ClientId"]
    let displayName = content["DisplayName"]
    let iconName    = content["IconName"]

    if (newClientId === clientId) {
        renderNewClientCard(newClientId, displayName, iconName, true)
        myDisplayNameInput = document.getElementById("my-display-name")

        // on change, broadcast new name to the other clients
        myDisplayNameInput.addEventListener("input", () => {
            let newDisplayName = myDisplayNameInput.value
            ws.send(JSON.stringify({ Type: NAME_CHANGE, Content: newDisplayName }))
        })

        // on focus, preselect the text for convenience
        myDisplayNameInput.addEventListener("focus", () => {
            myDisplayNameInput.select()
        })

        // on enter hit, remove focus from input for convenience
        myDisplayNameInput.addEventListener("keyup", e => {
            if (e.key === "Enter") {
                myDisplayNameInput.blur() // unfocus the element on enter
            }
        })

        // on unfocus, if the name is blank, reset it to the default name
        myDisplayNameInput.addEventListener("blur", () => {
            if (!myDisplayNameInput.value) {
                myDisplayNameInput.value = `Player ${clientId}`
                ws.send(JSON.stringify({ Type: NAME_CHANGE, Content: myDisplayNameInput.value }))
            }
        })

        // once joined, pre-select the text for convenience
        myDisplayNameInput.select()
    } else {
        renderNewClientCard(newClientId, displayName, iconName, false)
    }

    clientJoinedAudio.volume = VOLUME
    clientJoinedAudio.play()

    if (document.getElementById("clients-list").children.length >= 2) {
        startGameButton.textContent = "Start game!"
        startGameButton.removeAttribute("disabled")
        restartGameButton.removeAttribute("disabled")
    }
}

function onClientLeft(leavingClientId) {
    document.querySelector(`#clients-list [data-client-id="${leavingClientId}"]`).remove()
    if (document.getElementById("clients-list").children.length < 2) {
        startGameButton.textContent = "Waiting for players..."
        startGameButton.setAttribute("disabled", "")
        restartGameButton.setAttribute("disabled", "")
    }
}

function onNameChange(content) {
    let renamingClientId = content["ClientId"]
    let newDisplayName = content["NewDisplayName"]
    // for our own name change, the user has already updated the input,
    // so, only need to handle the case of other users changing their name
    if (renamingClientId !== clientId) {
        document.querySelector(`#clients-list [data-client-id="${renamingClientId}"] [data-display-name]`).textContent = newDisplayName
    }
}

function onClientsTurn(content) {
    clearInterval(turnCountdownInterval)

    startGameButton.classList.add("hidden")
    inviteButton.classList.add("hidden")

    let newClientsTurnId = content["ClientId"]
    let time = content["Time"]

    statusText.textContent = `Challenge: ${content["Challenge"]}   Time left: ${time}s`
    statusText.classList.remove("hidden")
    turnCountdownInterval = setInterval(() => {
        // sometimes, depending on timing, this may fire one more time after the game is over
        // so, don't update the status text if it's already declared a winner
        if (time > 0 && statusText.textContent.startsWith("Challenge")) {
            time--
            statusText.textContent = `Challenge: ${content["Challenge"]}   Time left: ${time}s`
        }
    }, 1_000)

    if (clientsTurnId) {
        let previousTurnClient = document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`)
        previousTurnClient.classList.add("invisible")
    }

    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess]`).textContent = ""
    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess-pill]`).classList.remove("invisible")

    if (clientId === newClientsTurnId) {
        // it's our turn
        answerInput.value = ""
        challengeInputSection.classList.remove("hidden")
        answerInput.focus()
    } else {
        // it's not our turn
        challengeInputSection.classList.add("hidden")
    }

    clientsTurnId = newClientsTurnId
}

function onAnswerPreview(answerPreview) {
    let answerPreviewText = answerPreview.length <= 20 ? answerPreview : answerPreview.substring(0, 20).concat("...")
    document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess]`).textContent = answerPreviewText
}

function onAnswerAccepted() {
    answerAcceptedAudio.volume = VOLUME
    answerAcceptedAudio.play()
}

function onAnswerRejected() {
    if (clientsTurnId === clientId) {
        shakeElement(answerInput, 20)
    }

    let pill = document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`)
    shakeElement(pill, 10)
}

function onTurnExpired(eliminatedClientId) {
    document.querySelector(`#clients-list [data-client-id="${eliminatedClientId}"]`).classList.add("opacity-40")
    clientEliminated.volume = VOLUME
    clientEliminated.play()
    if (eliminatedClientId === clientId) {
        challengeInputSection.classList.add("hidden")
    }
}

function onGameOver(winningClientId) {
    clearInterval(turnCountdownInterval)

    let winnersName;
    if (winningClientId === clientId) {
        // we won!
        winnersName = document.getElementById("my-display-name").value
    } else {
        winnersName = document.querySelector(`#clients-list [data-client-id="${winningClientId}"] [data-display-name]`).textContent
    }
    statusText.textContent = `🎉 ${winnersName} has won! 🎉`

    challengeInputSection.classList.add("hidden")
    restartGameButton.classList.remove("hidden")
    inviteButtonText.textContent = "Copy invite link"
    inviteButton.classList.remove("hidden")
}

function onRestartGame() {
    restartGameButton.classList.add("hidden")
    document.querySelectorAll("#clients-list [data-client-id]").forEach(renderedClient => {
        renderedClient.classList.remove("opacity-40")
    })
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