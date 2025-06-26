package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	// Connect to database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Test the connection
	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	fmt.Println("Connected to database successfully!")

	// Test the priority query
	testPriorityQuery(ctx, db)
}

func testPriorityQuery(ctx context.Context, db *sql.DB) {
	// First, let's check if we have any tasks with the new priority_score column
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = 'tasks' 
		AND column_name = 'priority_score'
	`).Scan(&count)
	
	if err != nil {
		log.Fatal("Failed to check for priority_score column:", err)
	}

	if count == 0 {
		fmt.Println("❌ priority_score column does not exist in tasks table")
		fmt.Println("The database schema needs to be updated first")
		return
	}

	fmt.Println("✅ priority_score column exists in tasks table")

	// Check if the priority index exists
	var indexCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM pg_indexes 
		WHERE tablename = 'tasks' 
		AND indexname = 'idx_tasks_priority'
	`).Scan(&indexCount)
	
	if err != nil {
		log.Fatal("Failed to check for priority index:", err)
	}

	if indexCount > 0 {
		fmt.Println("✅ idx_tasks_priority index exists")
	} else {
		fmt.Println("❌ idx_tasks_priority index does not exist")
	}

	// Test the GetNextTask query with priority ordering
	fmt.Println("\nTesting priority-based task selection...")
	
	// Create a test query similar to GetNextTask
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.path, t.priority_score, t.created_at
		FROM tasks t
		WHERE t.status = 'pending'
		ORDER BY t.priority_score DESC, t.created_at ASC
		LIMIT 5
	`)
	
	if err != nil {
		log.Fatal("Failed to query tasks:", err)
	}
	defer rows.Close()

	fmt.Println("\nTop 5 pending tasks by priority:")
	fmt.Println("ID | Path | Priority | Created")
	fmt.Println("---|------|----------|--------")
	
	taskCount := 0
	for rows.Next() {
		var id, path string
		var priority float64
		var created time.Time
		
		err := rows.Scan(&id, &path, &priority, &created)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		
		fmt.Printf("%s | %s | %.3f | %s\n", id[:8], path, priority, created.Format("15:04:05"))
		taskCount++
	}

	if taskCount == 0 {
		fmt.Println("No pending tasks found")
	}

	// Test the batch update query
	fmt.Println("\nTesting batch priority update query...")
	testPaths := []string{"/about", "/services", "/contact"}
	
	result, err := db.ExecContext(ctx, `
		SELECT COUNT(*) as count
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		WHERE p.path = ANY($1::text[])
	`, testPaths)
	
	if err != nil {
		fmt.Printf("❌ Batch query test failed: %v\n", err)
	} else {
		fmt.Println("✅ Batch query syntax is valid")
		_ = result
	}
}