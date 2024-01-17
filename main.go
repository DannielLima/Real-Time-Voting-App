package main

import (
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Estrutura para armazenar os votos
type Vote struct {
	Option    string
	Timestamp time.Time
}

// Estrutura para armazenar o estado da votação
type VotingState struct {
	Votes       []Vote
	Results     map[string]int
	Connections map[*websocket.Conn]bool
	Mutex       sync.Mutex
}

var votingState = VotingState{
	Results:     make(map[string]int),
	Connections: make(map[*websocket.Conn]bool),
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/ws", websocketHandler)

	go updateResults()

	log.Println("Servidor iniciado na porta :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Erro ao iniciar o servidor: ", err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("index").Parse(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Votação em Tempo Real</title>
		</head>
		<body>
			<h1>Votação em Tempo Real</h1>
			<form id="votingForm">
				<label for="optionA">Opção A</label>
				<input type="radio" name="option" value="A" required>

				<label for="optionB">Opção B</label>
				<input type="radio" name="option" value="B" required>

				<button type="submit">Votar</button>
			</form>

			<h2>Resultados:</h2>
			<ul id="results"></ul>

			<script>
				const socket = new WebSocket("ws://localhost:8080/ws");

				socket.addEventListener("message", function (event) {
					const results = JSON.parse(event.data);
					updateResults(results);
				});

				document.getElementById("votingForm").addEventListener("submit", function (event) {
					event.preventDefault();
					const formData = new FormData(event.target);
					const vote = formData.get("option");

					socket.send(JSON.stringify({ vote: vote }));
				});

				function updateResults(results) {
					const resultsList = document.getElementById("results");
					resultsList.innerHTML = "";

					for (const option in results) {
						const listItem = document.createElement("li");
						listItem.textContent = option + ": " + results[option] + " votos";

						resultsList.appendChild(listItem);
					}
				}
			</script>
		</body>
		</html>
	`)

	if err != nil {
		log.Println("Erro ao analisar o modelo:", err)
		return
	}

	tmpl.Execute(w, nil)
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	defer conn.Close()

	votingState.Mutex.Lock()
	votingState.Connections[conn] = true
	votingState.Mutex.Unlock()

	for {
		var msg map[string]string
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Erro ao ler mensagem:", err)
			votingState.Mutex.Lock()
			delete(votingState.Connections, conn)
			votingState.Mutex.Unlock()
			break
		}

		vote := Vote{
			Option:    msg["vote"],
			Timestamp: time.Now(),
		}

		votingState.Mutex.Lock()
		votingState.Votes = append(votingState.Votes, vote)
		votingState.Results[vote.Option]++
		votingState.Mutex.Unlock()

		updateResultsToClients()
	}
}

func updateResultsToClients() {
	votingState.Mutex.Lock()
	defer votingState.Mutex.Unlock()

	resultsJSON := make(map[string]int)
	for option, count := range votingState.Results {
		resultsJSON[option] = count
	}

	for conn := range votingState.Connections {
		err := conn.WriteJSON(resultsJSON)
		if err != nil {
			log.Println("Erro ao enviar mensagem:", err)
			conn.Close()
			delete(votingState.Connections, conn)
		}
	}
}

func updateResults() {
	for {
		time.Sleep(10 * time.Second) // Atualiza a cada 10 segundos
		updateResultsToClients()
	}
}
