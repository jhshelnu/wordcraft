# wordcraft

## About
wordcraft is an online multiplayer word game available at https://wordcraft.ing

Players must produce words given challenges which are pieces of words. For a challenge of "**st**", players can answer with:
- **st**riped
- je**st**er
- fea**st**
- or any other valid English word containing "**st**"!

Players compete against the clock and each other until one person is left standing.

Player icon images were provided for free by [freepik](https://freepik.com) and can be found [here](https://www.freepik.com/free-vector/cute-animal-icons-collection_1121413.htm)

## Technologies used
wordcraft uses a webserver written in Go which serves templated HTML and JS. Websockets are used to enable two-way communications between the client and server.

The UI is designed using [TailwindCSS](https://tailwindcss.com/) and [DaisyUI](https://daisyui.com/)

## Local development
Building and running wordcraft locally requires Go >=1.23 and a reasonably up-to-date `npm`

To get started, run Go's `generate` subcommand across the codebase to handle any code-generation needed before a build:

`go generate ./...`

Next, run the `npm` build script which will generate the final css file based on classes used in the application:

`npm install && npm run build`

Or, for development, use the `devBuild` script to enable hot-reloading of the css file:

`npm run devBuild`

And finally, building and running the application:

`go build -o app && ./app`

The server will listen on the port defined in the `PORT` environment variable, falling back to port 8080 as a default.

For local development, the websocket connection will be **insecure**, using the `ws` protocol instead of the secure `wss` protocol.
For production, the environment variable `PROD` needs to be set. It can be set to `1`, `true`, etc. Setting this will configure the webserver in production mode as well as switch the websocket protocol to the secure `wss` protocol.



## Todo
- add server timeout if no events occur within a time limit
- rate limit messages/lobby creation/etc
- outline player card when it's their turn, only show answer pill when current answer isn't empty
- add end game sound effects/confetti/etc
- allow players to change profile pictures
- gracefully handle browser refreshes (if possible: remember player)
- add lobby music with volume slider
- display a popup explaining the game rules before connecting to the lobby 
  - this will help new players who join via invite link understand how to play
  - this will also allow for music to be played right away when joining, since dismissing the popup counts as a DOM interaction
