document.addEventListener("DOMContentLoaded", () => {
    let createLobby = document.getElementById("create-lobby")

    createLobby.addEventListener("click", async () => {
        let res = await fetch("/api/lobby", { method: "POST" })
        if (!res.ok) {
            console.log(res)
            alert("Failed to create lobby. See the console for details")
            return
        }

        let lobbyId = (await res.json())["lobbyId"]
        if (lobbyId) {
            window.location.href = "/lobby/" + lobbyId
        }
    })
})