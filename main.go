package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jhshelnu/wordcraft/game"
	"github.com/jhshelnu/wordcraft/icons"
	"github.com/jhshelnu/wordcraft/words"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var isProd = os.Getenv("PROD") != ""

var logger = log.New(os.Stdout, "Application: ", log.Lshortfile|log.Lmsgprefix)

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
	c.HTML(http.StatusOK, "home.gohtml", gin.H{})
}

// navigates the user to the page for a specific lobby
func openLobby(c *gin.Context) {
	lobbyId, err := uuid.Parse(c.Param("lobbyId"))
	if err != nil {
		c.HTML(http.StatusOK, "home.gohtml", gin.H{
			"error": "Invalid lobby Id",
		})
		return
	}

	_, exists := lobbies[lobbyId]
	if !exists {
		c.HTML(http.StatusOK, "home.gohtml", gin.H{
			"error": "Lobby not found",
		})
		return
	}

	c.HTML(http.StatusOK, "lobby.gohtml", gin.H{"lobbyId": lobbyId, "isProd": isProd})
}

// once on the page for a specific lobby, the browser sends a request here to establish a WebSocket connection
// this is what actually causes the user to "join" the lobby and be able to play
func joinLobby(c *gin.Context) {
	lobbyId, err := uuid.Parse(c.Param("lobbyId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("failed to parse lobbyId: %v", err)})
		return
	}

	if _, exists := lobbies[lobbyId]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"message": "Lobby not found"})
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

// responds with the current time as milliseconds since the unix epoch (in UTC)
func getCurrentTime(c *gin.Context) {
	now := time.Now().UnixMilli()
	c.Data(http.StatusOK, gin.MIMEPlain, []byte(strconv.FormatInt(now, 10)))
}

func main() {
	if err := words.Init(); err != nil {
		log.Fatal(err)
	}

	if err := icons.Init(); err != nil {
		log.Fatal(err)
	}

	go handleEndedLobbies()

	if isProd {
		gin.SetMode(gin.ReleaseMode)
	}
	server := gin.New()

	// Static assets
	server.Static("/static", "./static")

	// API
	apiGroup := server.Group("/api")
	apiGroup.GET("/time", getCurrentTime)
	apiGroup.POST("/lobby", createLobby)

	// HTML
	server.LoadHTMLGlob("templates/*.gohtml")
	server.GET("/", handleIndex)
	server.GET("/lobby/:lobbyId", openLobby)

	// WebSocket
	server.GET("/ws/:lobbyId", joinLobby)

	go func() {
		err := server.Run()
		if err != nil {
			log.Fatalf("Failed to start application server: %v", err)
		}
	}()

	shutdownRequested := make(chan os.Signal)
	signal.Notify(shutdownRequested, syscall.SIGTERM, syscall.SIGINT)

	<-shutdownRequested
	if len(lobbies) == 0 {
		logger.Printf("Received request to shutdown. No lobbies in progress. Goodbye.")
		os.Exit(0)
	}

	logger.Printf("Received request to shutdown. Notifying %d lobbies first. Goodbye.", len(lobbies))
	for _, lobby := range lobbies {
		lobby.BroadcastShutdown()
	}
	time.Sleep(8 * time.Second) // give the clients enough time to see the shutdown message and be redirected to the home screen
	os.Exit(0)
}
