package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	opCodeMove   int64 = 1
	opCodeState  int64 = 2
	opCodeResult int64 = 3
)

func checkWin(grid [3][3]int) int {
	// Check rows and columns.
	for i := 0; i < 3; i++ {
		if grid[i][0] != 0 && grid[i][0] == grid[i][1] && grid[i][1] == grid[i][2] {
			return grid[i][0]
		}
		if grid[0][i] != 0 && grid[0][i] == grid[1][i] && grid[1][i] == grid[2][i] {
			return grid[0][i]
		}
	}

	// Check diagonals.
	if grid[0][0] != 0 && grid[0][0] == grid[1][1] && grid[1][1] == grid[2][2] {
		return grid[0][0]
	}
	if grid[0][2] != 0 && grid[0][2] == grid[1][1] && grid[1][1] == grid[2][0] {
		return grid[0][2]
	}

	return 0
}

func checkdraw(grid [3][3]int) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if grid[i][j] == 0 {
				return false
			}
		}
	}

	return true
}

type Match struct{}

type PlayerStats struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
}

type MatchPlayer struct {
	UserID   string
	Username string
}

type MatchState struct {
	grid     [3][3]int // 0 = empty, 1 = X, 2 = O
	Players  []MatchPlayer
	turn     int
	gameOver bool
	winner   int
	draw     bool
}

type MovePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type PlayerView struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Symbol   string `json:"symbol"`
	Wins     int    `json:"wins,omitempty"`
	Losses   int    `json:"losses,omitempty"`
	Draws    int    `json:"draws,omitempty"`
}

type StatePayload struct {
	Grid            [3][3]int    `json:"grid"`
	Turn            int          `json:"turn"`
	CurrentPlayerID string       `json:"current_player_id,omitempty"`
	GameOver        bool         `json:"game_over"`
	Winner          int          `json:"winner"`
	Draw            bool         `json:"draw"`
	PlayerCount     int          `json:"player_count"`
	Players         []PlayerView `json:"players"`
}

type ResultPayload struct {
	Winner         int          `json:"winner"`
	Draw           bool         `json:"draw"`
	WinnerUserID   string       `json:"winner_user_id,omitempty"`
	WinnerUsername string       `json:"winner_username,omitempty"`
	Players        []PlayerView `json:"players"`
}

func symbolForPlayer(index int) string {
	if index == 0 {
		return "X"
	}

	return "O"
}

func readPlayerStats(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerStats, error) {
	reads := []*runtime.StorageRead{
		{
			Collection: "player_stats",
			Key:        "summary",
			UserID:     userID,
		},
	}

	objects, err := nk.StorageRead(ctx, reads)
	if err != nil {
		return PlayerStats{}, err
	}

	if len(objects) == 0 {
		return PlayerStats{}, nil
	}

	var stats PlayerStats
	if err := json.Unmarshal([]byte(objects[0].Value), &stats); err != nil {
		return PlayerStats{}, err
	}

	return stats, nil
}

func writePlayerStats(ctx context.Context, nk runtime.NakamaModule, userID string, stats PlayerStats) error {
	value, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	writes := []*runtime.StorageWrite{
		{
			Collection:      "player_stats",
			Key:             "summary",
			UserID:          userID,
			Value:           string(value),
			PermissionRead:  runtime.STORAGE_PERMISSION_OWNER_READ,
			PermissionWrite: runtime.STORAGE_PERMISSION_NO_WRITE,
		},
	}

	_, err = nk.StorageWrite(ctx, writes)
	return err
}

func buildPlayerViews(state *MatchState, statsMap map[string]PlayerStats) []PlayerView {
	views := make([]PlayerView, 0, len(state.Players))
	for i, player := range state.Players {
		view := PlayerView{
			UserID:   player.UserID,
			Username: player.Username,
			Symbol:   symbolForPlayer(i),
		}

		if statsMap != nil {
			if stats, ok := statsMap[player.UserID]; ok {
				view.Wins = stats.Wins
				view.Losses = stats.Losses
				view.Draws = stats.Draws
			}
		}

		views = append(views, view)
	}

	return views
}

func broadcastState(logger runtime.Logger, dispatcher runtime.MatchDispatcher, state *MatchState) {
	currentPlayerID := ""
	if len(state.Players) >= state.turn {
		currentPlayerID = state.Players[state.turn-1].UserID
	}

	payload := StatePayload{
		Grid:            state.grid,
		Turn:            state.turn,
		CurrentPlayerID: currentPlayerID,
		GameOver:        state.gameOver,
		Winner:          state.winner,
		Draw:            state.draw,
		PlayerCount:     len(state.Players),
		Players:         buildPlayerViews(state, nil),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Could not marshal match state: %v", err)
		return
	}

	if err := dispatcher.BroadcastMessage(opCodeState, data, nil, nil, true); err != nil {
		logger.Error("Could not broadcast state: %v", err)
	}
}

func finalizeGame(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, state *MatchState) {
	statsByUser := make(map[string]PlayerStats, len(state.Players))
	winnerUserID := ""
	winnerUsername := ""

	for i, player := range state.Players {
		stats, err := readPlayerStats(ctx, nk, player.UserID)
		if err != nil {
			logger.Error("Could not read stats for %s: %v", player.UserID, err)
			stats = PlayerStats{}
		}

		if state.draw {
			stats.Draws++
		} else if state.winner == i+1 {
			stats.Wins++
			winnerUserID = player.UserID
			winnerUsername = player.Username
		} else {
			stats.Losses++
		}

		if err := writePlayerStats(ctx, nk, player.UserID, stats); err != nil {
			logger.Error("Could not write stats for %s: %v", player.UserID, err)
		}

		statsByUser[player.UserID] = stats
	}

	broadcastState(logger, dispatcher, state)

	result := ResultPayload{
		Winner:         state.winner,
		Draw:           state.draw,
		WinnerUserID:   winnerUserID,
		WinnerUsername: winnerUsername,
		Players:        buildPlayerViews(state, statsByUser),
	}

	data, err := json.Marshal(result)
	if err != nil {
		logger.Error("Could not marshal result payload: %v", err)
		return
	}

	if err := dispatcher.BroadcastMessage(opCodeResult, data, nil, nil, true); err != nil {
		logger.Error("Could not broadcast result payload: %v", err)
	}
}

func parseMovePayload(data []byte) (int, int, bool) {
	var move MovePayload
	if err := json.Unmarshal(data, &move); err == nil {
		return move.Row, move.Col, true
	}

	if len(data) < 2 {
		return 0, 0, false
	}

	row := int(data[0] - '0')
	col := int(data[1] - '0')
	return row, col, true
}

func (m *Match) MatchInit(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	state := &MatchState{
		grid:     [3][3]int{},
		Players:  []MatchPlayer{},
		turn:     1,
		gameOver: false,
	}

	logger.Info("MatchInit called")

	return state, 1, ""
}

func (m *Match) MatchJoinAttempt(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presence runtime.Presence, metadata map[string]string) (interface{}, bool, string) {
	gameState := state.(*MatchState)

	for _, player := range gameState.Players {
		if player.UserID == presence.GetUserId() {
			return gameState, true, ""
		}
	}

	if len(gameState.Players) >= 2 {
		return gameState, false, "match is full"
	}

	return gameState, true, ""
}

func (m *Match) MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	gameState := state.(*MatchState)

	for _, p := range presences {
		alreadyJoined := false
		for _, player := range gameState.Players {
			if player.UserID == p.GetUserId() {
				alreadyJoined = true
				break
			}
		}
		if alreadyJoined || len(gameState.Players) >= 2 {
			continue
		}

		gameState.Players = append(gameState.Players, MatchPlayer{
			UserID:   p.GetUserId(),
			Username: p.GetUsername(),
		})
	}

	logger.Info("Players joined: %v", gameState.Players)
	broadcastState(logger, dispatcher, gameState)

	return gameState
}

func (m *Match) MatchLoop(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, messages []runtime.MatchData) interface{} {
	gameState := state.(*MatchState)

	for _, msg := range messages {
		logger.Info("Received message: %s", string(msg.GetData()))

		if msg.GetOpCode() != opCodeMove {
			logger.Info("Ignoring op code %d", msg.GetOpCode())
			continue
		}

		if gameState.gameOver {
			logger.Info("Ignoring move because the game is already over")
			continue
		}

		if len(gameState.Players) < 2 {
			logger.Info("Ignoring move because two players have not joined yet")
			continue
		}

		row, col, ok := parseMovePayload(msg.GetData())
		if !ok {
			logger.Info("Ignoring invalid move payload from user %s", msg.GetUserId())
			continue
		}

		if row < 0 || row > 2 || col < 0 || col > 2 {
			logger.Info("Ignoring out-of-range move from user %s: row=%d col=%d", msg.GetUserId(), row, col)
			continue
		}

		expectedPlayerID := gameState.Players[gameState.turn-1].UserID
		if msg.GetUserId() != expectedPlayerID {
			logger.Info("Ignoring out-of-turn move from user %s", msg.GetUserId())
			continue
		}

		if gameState.grid[row][col] != 0 {
			logger.Info("Ignoring move to occupied cell row=%d col=%d", row, col)
			continue
		}

		gameState.grid[row][col] = gameState.turn

		win := checkWin(gameState.grid)
		if win != 0 {
			gameState.winner = win
			gameState.gameOver = true
			logger.Info("Player %s wins!", gameState.Players[win-1].UserID)
			finalizeGame(ctx, logger, nk, dispatcher, gameState)
			continue
		}

		if checkdraw(gameState.grid) {
			gameState.draw = true
			gameState.gameOver = true
			logger.Info("Game is a draw")
			finalizeGame(ctx, logger, nk, dispatcher, gameState)
			continue
		}

		if gameState.turn == 1 {
			gameState.turn = 2
		} else {
			gameState.turn = 1
		}

		broadcastState(logger, dispatcher, gameState)
	}

	logger.Info("MatchLoop running, messages: %d", len(messages))

	return gameState
}

func (m *Match) MatchLeave(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	logger.Info("Player left")
	return state
}

func (m *Match) MatchTerminate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, graceSeconds int) interface{} {
	logger.Info("Match terminated")
	return state
}

func (m *Match) MatchSignal(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, data string) (interface{}, string) {
	logger.Info("Match signal received: %s", data)
	return state, data
}
