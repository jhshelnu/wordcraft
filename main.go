package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jhshelnu/wordcraft/game"
	"github.com/jhshelnu/wordcraft/icons"
	"github.com/jhshelnu/wordcraft/words"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/sethvargo/go-diceware/diceware"
)

const MaxLobbySize = 10

var isProd = os.Getenv("PROD") != ""

var logger = log.New(os.Stdout, "Application: ", log.Lshortfile|log.Lmsgprefix)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var lobbies = cmap.New[*game.Lobby]() // concurrent hash map, better optimized than sync.Map
var lobbyEndChan = make(chan string)

func generateNewId() string {
	attempts := 0
	for {
		// start with 2 words, every 1 thousand attempts add another word
		words, err := diceware.GenerateWithWordList(2+(attempts/1_000), diceware.WordListEffSmall())
		if err != nil {
			panic(err)
		}

		id := strings.Join(words, "-")
		if !lobbies.Has(id) {
			return id
		}

		attempts++
	}
}

func createLobby(c *gin.Context) {
	lobby := game.NewLobby(generateNewId(), lobbyEndChan)
	go lobby.StartLobby()
	lobbies.Set(lobby.Id, lobby)
	c.JSON(http.StatusCreated, gin.H{"lobbyId": lobby.Id})
}

func handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "home.gohtml", gin.H{})
}

// navigates the user to the page for a specific lobby
func openLobby(c *gin.Context) {
	lobbyId := c.Param("lobbyId")

	lobby, exists := lobbies.Get(lobbyId)
	if !exists {
		c.HTML(http.StatusOK, "home.gohtml", gin.H{
			"error": "Lobby not found",
		})
		return
	}

	if lobby.GetClientCount() >= MaxLobbySize {
		c.HTML(http.StatusOK, "home.gohtml", gin.H{
			"error": "Lobby is full",
		})
		return
	}

	c.HTML(http.StatusOK, "lobby.gohtml", gin.H{"lobbyId": lobbyId, "isProd": isProd})
}

// once on the page for a specific lobby, the browser sends a request here to establish a WebSocket connection
// this is what actually causes the user to "join" the lobby and be able to play
func joinLobby(c *gin.Context) {
	lobbyId := c.Param("lobbyId")
	reconnectToken := c.Query("reconnectToken")

	lobby, exists := lobbies.Get(lobbyId)
	if !exists {
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

	err = game.JoinConnToLobby(conn, lobby, reconnectToken)
	if err != nil {
		fmt.Printf("Client failed to join lobby: %v\n", err)
		_ = conn.Close()
	}
}

func handleEndedLobbies() {
	for {
		endedLobbyId := <-lobbyEndChan
		lobbies.Remove(endedLobbyId)
	}
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

	shutdownRequested := make(chan os.Signal, 1)
	signal.Notify(shutdownRequested, syscall.SIGTERM, syscall.SIGINT)

	<-shutdownRequested
	if lobbies.Count() == 0 {
		logger.Printf("Received request to shutdown. No lobbies in progress. Goodbye.")
		os.Exit(0)
	}

	logger.Printf("Received request to shutdown. Notifying %d lobbies first. Goodbye.", lobbies.Count())
	lobbies.IterCb(func(_ string, lobby *game.Lobby) {
		lobby.BroadcastShutdown()
	})
	time.Sleep(8 * time.Second) // give the clients enough time to see the shutdown message and be redirected to the home screen
	os.Exit(0)
}
