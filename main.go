package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log"
	"maps"
	"net/http"
	"slices"
)

var lobbies = make(map[uuid.UUID]bool)

func createLobby(c *gin.Context) {
	lobbyId := uuid.New()
	lobbies[lobbyId] = true
	c.JSON(http.StatusCreated, gin.H{
		"lobbyId": lobbyId,
	})
}

func listLobbies(c *gin.Context) {
	c.JSON(http.StatusOK, slices.AppendSeq(make([]uuid.UUID, 0, len(lobbies)), maps.Keys(lobbies)))
}

func handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.gohtml", gin.H{})
}

func serveWS(c *gin.Context) {
	lobbyId, err := uuid.Parse(c.Param("lobbyId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Failed to parse lobbyId: %v", err)})
		return
	}

	if !lobbies[lobbyId] {
		c.JSON(http.StatusNotFound, gin.H{"message": "Lobby not found"})
		return
	}

	// todo: establish ws connection here
	log.Printf("Received request to establish ws connection for lobby [%s]", lobbyId)
}

func main() {
	server := gin.New()

	// Static assets
	server.Static("/static", "./static")

	// API
	apiGroup := server.Group("/api")
	apiGroup.POST("/lobby", createLobby)
	apiGroup.GET("/lobbies", listLobbies)

	// HTML
	server.LoadHTMLGlob("templates/*")
	server.GET("/", handleIndex)

	// WebSocket
	server.GET("/ws/:lobbyId", serveWS)

	err := server.Run(":8080")
	if err != nil {
		log.Fatalf("Failed to start application server: %v", err)
	}
}
