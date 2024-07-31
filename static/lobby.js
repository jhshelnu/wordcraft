document.addEventListener("DOMContentLoaded", () => {
    let ws = new WebSocket(`ws://localhost:8080/ws/${lobbyId}`)

    let sendButton = document.getElementById("send-button")
    let input = document.getElementById("send-text")

    ws.onmessage = ({ data }) => {
        alert(data)
    }

    sendButton.addEventListener("click", () => {
        if (ws.readyState !== WebSocket.OPEN) {
            return
        }
        ws.send(input.value)
    })
})