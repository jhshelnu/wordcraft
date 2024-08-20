# wordgame

## About
todo

## Technologies used
todo

## Local development
`go install golang.org/x/tools/cmd/stringer@latest`
`go generate ./...`
`go build`

## Todo
- fill out sections in this readme
- cap lobby size at 10
- outline player card when it's their turn, only show answer pill when current answer isn't empty
- handle end game announcement/effects
- allow players to change profile pictures
- display potential answers when a player gets eliminated
- gracefully handle browser refreshes (if possible: remember player)
- add lobby music with volume slider
- allow mid-game joining
- update game description on home page to reflect all the rules
- display a toast if a word gets rejected because it has been used before (or just remove this requirement altogether, doesn't seem very impactful)
- display a popup explaining the game rules before connecting to the lobby 
  - this will help new players who join via invite link understand how to play
  - this will also allow for music to be played right away when joining, since dismissing the popup counts as a DOM interaction