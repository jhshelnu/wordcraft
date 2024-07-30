document.addEventListener("DOMContentLoaded", () => {
    let ws = new WebSocket("wss://localhost:8080/ws/123")

    let sendButton = document.getElementById("send-button")
    let input = document.getElementById("send-text")

    sendButton.addEventListener('click', () => {
        if (ws.readyState === WebSocket.OPEN) {
            console.log(`going to send: "${input.value}"`)
            ws.send(input.value)
            console.log('sent!')
        }
    })
})