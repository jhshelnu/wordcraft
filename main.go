package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jhshelnu/wordgame/game"
	"log"
	"net/http"
)

type HttpError struct {
	status  int
	message string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var lobbies = make(map[uuid.UUID]*game.Lobby)
var lobbyEnded = make(chan uuid.UUID)

func createLobby(c *gin.Context) {
	lobby := game.NewLobby(lobbyEnded)
	go lobby.StartLobby()
	lobbies[lobby.Id] = lobby
	c.JSON(http.StatusCreated, gin.H{"lobbyId": lobby.Id})
}

func handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.gohtml", gin.H{})
}

func getValidLobbyId(lobbyIdStr string) (uuid.UUID, *HttpError) {
	lobbyId, err := uuid.Parse(lobbyIdStr)
	if err != nil {
		return uuid.UUID{}, &HttpError{status: http.StatusBadRequest, message: fmt.Sprintf("failed to parse lobbyId: %v", err)}
	}

	if _, exists := lobbies[lobbyId]; !exists {
		return uuid.UUID{}, &HttpError{status: http.StatusNotFound, message: "Lobby not found"}
	}

	return lobbyId, nil
}

// navigates the user to the page for a specific lobby
func openLobby(c *gin.Context) {
	lobbyId, httpError := getValidLobbyId(c.Param("lobbyId"))
	if httpError != nil {
		c.JSON(httpError.status, gin.H{"message": httpError.message})
		return
	}
	c.HTML(http.StatusOK, "lobby.gohtml", gin.H{"lobbyId": lobbyId})
}

// once on the page for a specific lobby, the browser sends a request here to establish a WebSocket connection
// this is what actually causes the user to "join" the lobby and be able to play
func joinLobby(c *gin.Context) {
	lobbyId, httpError := getValidLobbyId(c.Param("lobbyId"))
	if httpError != nil {
		c.JSON(httpError.status, gin.H{"message": httpError.message})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade ws connection: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to join lobby. An unknown error occurred when upgrading to a websocket connection.",
		})
		return
	}

	err = game.JoinClientToLobby(conn, lobbies[lobbyId])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to join lobby. The connection was not properly added to the lobby."})
		return
	}
}

func handleEndedLobbies() {
	for {
		endedLobbyId := <-lobbyEnded
		delete(lobbies, endedLobbyId)
	}
}

func main() {
	go handleEndedLobbies()

	gin.SetMode(gin.ReleaseMode)
	server := gin.New()

	// Static assets
	server.Static("/static", "./static")

	// API
	apiGroup := server.Group("/api")
	apiGroup.POST("/lobby", createLobby)

	// HTML
	server.LoadHTMLGlob("templates/*")
	server.GET("/", handleIndex)
	server.GET("/lobby/:lobbyId", openLobby)

	// WebSocket
	server.GET("/ws/:lobbyId", joinLobby)

	log.Println("Starting server on port 8080")
	err := server.Run(":8080")
	if err != nil {
		log.Fatalf("Failed to start application server: %v", err)
	}
}
