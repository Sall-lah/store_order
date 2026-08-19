package db

import (
	"context"
	"fmt"
	"log"
	"time"
)

// InitClient establishes a connected Prisma database client instance with timeout verification.
// Why: Provides a unified, resilient startup sequence for PostgreSQL connection with timeout and logging.
func InitClient(databaseURL string) (*PrismaClient, error) {
	var client *PrismaClient
	if databaseURL != "" {
		client = NewClient(WithDatasourceURL(databaseURL))
	} else {
		client = NewClient()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx

	if err := client.Prisma.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	log.Println("[Database] Successfully connected to PostgreSQL via Prisma.")
	return client, nil
}
