document.addEventListener("DOMContentLoaded", () => {
    let createLobby = document.getElementById("create-lobby")

    createLobby.addEventListener("click", async () => {
        let res = await fetch("/api/lobby", { method: "POST" })
        if (!res.ok) {
            alert("Failed to create lobby. See the console for details")
            return
        }

        let lobbyId = (await res.json())["lobbyId"]
        if (!lobbyId) {
            alert("Failed to create lobby.")
            return
        }

        window.location.href = "/lobby/" + lobbyId
    })
})