// noinspection JSUnresolvedReference - GoLand doesn't recognize global objects declared in other script files like gsap

// message types
const START_GAME     = "start_game"      // the game has started
const CLIENT_DETAILS = "client_details"  // sent to a newly connected client, indicating their id
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

// different values for gameStatus that indicate what point we're at in the game
const WAITING_FOR_PLAYERS = 0
const IN_PROGRESS = 1
const OVER = 2

let ws                    // the websocket connection
let myClientId            // our assigned id for the lobby we're joining
let myDisplayNameInput    // the <input> which holds our current displayName
let startGameButton       // the button to start the game
let restartGameButton     // the button to restart the game
let inviteButton          // the button that copies the lobby link to the clipboard
let inviteButtonText      // the text of the invite button (changes after being clicked)
let clientsTurnId         // the id of the client whose turn it is
let challengeInputSection // the part of the page to get the user's input (only shown during their turn)
let answerInput           // the input element which holds what the user has typed so far
let statusText            // large text at the top of the screen displaying the current status (current challenge, who won, etc.)
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
    myClientId = content["ClientId"] // this is our assigned clientId for the rest of the lobby
    let status = content["Status"]   // the status of the game (need to know if it's started yet or not)
    let clients = content["Clients"] // all the clients that are already in the game
    let currentTurnId = content["CurrentTurnId"] // the id of the client whose turn it is (or 0 if not applicable)
    let currentChallenge = content["CurrentChallenge"] // what the current challenge is, or "" if there isn't one
    let turnEndTimestamp = content["TurnEnd"] // UTC // the timestamp in UTC for when the current turn expires

    // render the clients
    clients.forEach(client => {
        renderNewClientCard(client["Id"], client["DisplayName"], client["IconName"], client["Alive"], false)
    })

    // then render the other buttons, etc. depending on the game state
    switch (status) {
        case WAITING_FOR_PLAYERS:
            startGameButton.classList.remove("hidden")
            inviteButton.classList.remove("hidden")
            break
        case IN_PROGRESS:
            if (currentTurnId) {
                document.querySelector(`[data-client-id="${currentTurnId}"] [data-current-guess]`).textContent = ""
                document.querySelector(`[data-client-id="${currentTurnId}"] [data-current-guess-pill]`).classList.remove("invisible")
                clientsTurnId = currentTurnId
            }

            if (currentChallenge && turnEndTimestamp) {
                countDownTurn(currentChallenge, turnEndTimestamp)
            }
            break
        case OVER:
            restartGameButton.classList.remove("hidden")
            inviteButton.classList.remove("hidden")
            break
    }
}

function onClientJoined(content) {
    let newClientId = content["ClientId"]
    let displayName = content["DisplayName"]
    let iconName    = content["IconName"]
    let isAlive     = content["Alive"]

    if (newClientId !== myClientId) {
        // if the new client is not us, this is easy
        renderNewClientCard(newClientId, displayName, iconName, isAlive, false)
    } else {
        // if this is us, we do have some setup to do like registering event handlers
        renderNewClientCard(newClientId, displayName, iconName, isAlive, true)
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
                myDisplayNameInput.value = `Player ${myClientId}`
                ws.send(JSON.stringify({ Type: NAME_CHANGE, Content: myDisplayNameInput.value }))
            }
        })

        // once joined, pre-select the text for convenience
        myDisplayNameInput.select()
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
    if (renamingClientId !== myClientId) {
        document.querySelector(`#clients-list [data-client-id="${renamingClientId}"] [data-display-name]`).textContent = newDisplayName
    }
}

function renderNewClientCard(clientId, displayName, iconName, alive, isMe) {
    let clientsList = document.getElementById("clients-list")
    let template = document.createElement("template")
    template.innerHTML = `
        <div data-client-id="${clientId}" class="card card-compact bg-base-100 w-52 shadow-2xl ${!alive ? "opacity-40" : ""}">
            <img
                class="mask max-w-36 mx-auto"
                src="/static/icons/${iconName}"
                alt="${iconName}" />
            <div class="card-body items-center">
                ${isMe
                    ? `<input id="my-display-name" class="input card-title text-center w-44" value="${displayName}">`
                    : `<p data-display-name class="card-title">${displayName}</p>`
                }
                <div data-current-guess-pill class="rounded-full min-w-24 h-8 leading-8 bg-secondary text-center invisible">
                    <p data-current-guess class="font-bold px-3" style="color: oklch(var(--sc))"></p>
                </div>
            </div>
        </div>
    `
    clientsList.appendChild(template.content)
}

function onClientsTurn(content) {
    clearInterval(turnCountdownInterval)

    startGameButton.classList.add("hidden")
    inviteButton.classList.add("hidden")

    let newClientsTurnId = content["ClientId"]
    let turnEndTimestamp = content["TurnEnd"] // UTC
    let currentChallenge = content["Challenge"]

    countDownTurn(currentChallenge, turnEndTimestamp)

    if (clientsTurnId) {
        let previousTurnClient = document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`)
        if (previousTurnClient) { // can be null if the client in question left
            previousTurnClient.classList.add("invisible")
        }
    }

    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess]`).textContent = ""
    document.querySelector(`[data-client-id="${newClientsTurnId}"] [data-current-guess-pill]`).classList.remove("invisible")

    if (myClientId === newClientsTurnId) {
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

function countDownTurn(currentChallenge, turnEndTimestamp) {
    statusText.textContent = `Challenge: ${currentChallenge}   Time left: ${getSecondsUntil(turnEndTimestamp)}s`
    statusText.classList.remove("hidden")
    turnCountdownInterval = setInterval(() => {
        // sometimes, depending on timing, this may fire one more time after the game is over
        // so, don't update the status text if it's already declared a winner
        if (statusText.textContent.startsWith("Challenge")) {
            statusText.textContent = `Challenge: ${currentChallenge}   Time left: ${getSecondsUntil(turnEndTimestamp)}s`
        } else {
            clearInterval(turnCountdownInterval)
        }
    }, 1_000)
}

// returns the seconds until a given UTC timestamp, or 0 if the timestamp has already passed
function getSecondsUntil(utcTimestamp) {
    let secondsUntilTurnExpires = (new Date(utcTimestamp) - new Date()) / 1_000
    return Math.max(Math.floor(secondsUntilTurnExpires), 0)
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
    if (clientsTurnId === myClientId) {
        shakeElement(answerInput, 20)
    }

    let pill = document.querySelector(`[data-client-id="${clientsTurnId}"] [data-current-guess-pill]`)
    shakeElement(pill, 10)
}

function onTurnExpired(eliminatedClientId) {
    document.querySelector(`#clients-list [data-client-id="${eliminatedClientId}"]`).classList.add("opacity-40")
    clientEliminated.volume = VOLUME
    clientEliminated.play()
    if (eliminatedClientId === myClientId) {
        challengeInputSection.classList.add("hidden")
    }
}

function onGameOver(winningClientId) {
    clearInterval(turnCountdownInterval)

    let winnersName;
    if (winningClientId === myClientId) {
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