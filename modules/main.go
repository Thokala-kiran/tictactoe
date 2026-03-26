package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	if err := initializer.RegisterMatch("tic-tac-toe", func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
		return &Match{}, nil
	}); err != nil {
		return err
	}

	logger.Info("Nakama module loaded and tic-tac-toe registered")

	if err := initializer.RegisterMatchmakerMatched(func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, entries []runtime.MatchmakerEntry) (string, error) {
		matchID, err := nk.MatchCreate(ctx, "tic-tac-toe", nil)
		if err != nil {
			return "", err
		}

		logger.Info("Matchmaker created match: %s", matchID)
		return matchID, nil
	}); err != nil {
		return err
	}

	if err := initializer.RegisterRpc("create_match", func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
		matchID, err := nk.MatchCreate(ctx, "tic-tac-toe", nil)
		if err != nil {
			return "", err
		}

		logger.Info("Match created: %s", matchID)

		return matchID, nil
	}); err != nil {
		return err
	}

	if err := initializer.RegisterRpc("get_stats", func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
		userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
		if !ok || userID == "" {
			return "", runtime.NewError("user id missing from context", 13)
		}

		stats, err := readPlayerStats(ctx, nk, userID)
		if err != nil {
			return "", err
		}

		out, err := json.Marshal(stats)
		if err != nil {
			return "", err
		}

		return string(out), nil
	}); err != nil {
		return err
	}

	return nil
}
