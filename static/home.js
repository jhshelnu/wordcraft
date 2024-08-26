document.addEventListener("DOMContentLoaded", () => {
    let createLobby = document.getElementById("create-lobby")

    createLobby.addEventListener("click", async () => {
        let res = await fetch("/api/lobby", { method: "POST" })
        let body = await res.json()
        if (!res.ok) {
            console.log(res)
            const msg = `Failed to create lobby. ${body["message"] ?? "Unknown error. See console for details."}`
            toast(msg, "alert-error")
            return
        }

        let lobbyId = body["lobbyId"]
        if (lobbyId) {
            window.location.href = "/lobby/" + lobbyId
        }
    })
})