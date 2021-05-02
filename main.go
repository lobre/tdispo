package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	db := NewDB("cocorico.db")
	if err := db.Open(); err != nil {
		return err
	}

	statusService := NewStatusService(db)

	statuses, _, err := statusService.FindStatuses(ctx)
	if err != nil {
		return err
	}

	fmt.Println("list of statuses:")
	for _, s := range statuses {
		fmt.Printf("%d: %s\n", s.ID, s.Label)
	}

	for _, toAdd := range []string{"created", "canceled", "ready"} {
		exists := false
		for _, status := range statuses {
			if status.Label == toAdd {
				exists = true
			}
		}

		if !exists {
			newStatus := Status{Label: toAdd}
			err := statusService.CreateStatus(ctx, &newStatus)
			if err != nil {
				return err
			}

			fmt.Printf("new status %d: %s created\n", newStatus.ID, newStatus.Label)
		}
	}

	if err := db.Close(); err != nil {
		return err
	}

	return nil
}
