package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLConnection test MySQL connection
func TestMySQLConnection(t *testing.T) {
	// Connect to MySQL
	dsn := "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skip("MySQL is not available:", err)
		return
	}
	defer db.Close()

	// Set connection pool parameters
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx := context.Background()
	err = db.PingContext(ctx)
	require.NoError(t, err)

	// Create test table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			username VARCHAR(50) NOT NULL,
			email VARCHAR(100) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_username (username),
			INDEX idx_email (email)
		)
	`)
	require.NoError(t, err)

	// Clean up test data
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_users")

	// Test CRUD operations
	t.Run("CRUD_Operations", func(t *testing.T) {
		// INSERT
		result, err := db.ExecContext(ctx,
			"INSERT INTO test_users (username, email) VALUES (?, ?)",
			"testuser", "test@example.com",
		)
		assert.NoError(t, err)

		id, err := result.LastInsertId()
		assert.NoError(t, err)
		assert.Greater(t, id, int64(0))

		// SELECT
		var username, email string
		var createdAt, updatedAt time.Time
		err = db.QueryRowContext(ctx,
			"SELECT username, email, created_at, updated_at FROM test_users WHERE id = ?",
			id,
		).Scan(&username, &email, &createdAt, &updatedAt)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", username)
		assert.Equal(t, "test@example.com", email)

		// UPDATE
		_, err = db.ExecContext(ctx,
			"UPDATE test_users SET email = ? WHERE id = ?",
			"newemail@example.com", id,
		)
		assert.NoError(t, err)

		// Verify update
		err = db.QueryRowContext(ctx,
			"SELECT email FROM test_users WHERE id = ?",
			id,
		).Scan(&email)
		assert.NoError(t, err)
		assert.Equal(t, "newemail@example.com", email)

		// DELETE
		result, err = db.ExecContext(ctx, "DELETE FROM test_users WHERE id = ?", id)
		assert.NoError(t, err)

		rowsAffected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
	})

	// Test transactions
	t.Run("Transaction", func(t *testing.T) {
		// Begin transaction
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		// Insert multiple records
		for i := 1; i <= 3; i++ {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO test_users (username, email) VALUES (?, ?)",
				fmt.Sprintf("user%d", i),
				fmt.Sprintf("user%d@example.com", i),
			)
			require.NoError(t, err)
		}

		// Commit transaction
		err = tx.Commit()
		assert.NoError(t, err)

		// Verify data
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)

		// Clean up
		db.ExecContext(ctx, "DELETE FROM test_users")
	})

	// Test prepared statements
	t.Run("PreparedStatements", func(t *testing.T) {
		// Prepare insert statement
		stmt, err := db.PrepareContext(ctx,
			"INSERT INTO test_users (username, email) VALUES (?, ?)",
		)
		require.NoError(t, err)
		defer stmt.Close()

		// Batch insert
		for i := 1; i <= 10; i++ {
			_, err := stmt.ExecContext(ctx,
				fmt.Sprintf("prepared_user%d", i),
				fmt.Sprintf("prepared%d@example.com", i),
			)
			assert.NoError(t, err)
		}

		// Verify
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_users WHERE username LIKE 'prepared%'").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 10, count)

		// Clean up
		db.ExecContext(ctx, "DELETE FROM test_users WHERE username LIKE 'prepared%'")
	})
}

// TestMySQLConnectionPool test MySQL connection pool
func TestMySQLConnectionPool(t *testing.T) {
	dsn := "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skip("MySQL is not available:", err)
		return
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	ctx := context.Background()

	// Get initial statistics
	stats := db.Stats()
	t.Logf("Initial pool stats: Open=%d, Idle=%d, InUse=%d",
		stats.OpenConnections, stats.Idle, stats.InUse)

	// Concurrent query test
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			var result int
			err := db.QueryRowContext(ctx, "SELECT ?", id).Scan(&result)
			assert.NoError(t, err)
			assert.Equal(t, id, result)
			done <- true
		}(i)
	}

	// Wait for all queries to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check connection pool statistics
	stats = db.Stats()
	t.Logf("After concurrent queries: Open=%d, Idle=%d, InUse=%d, WaitCount=%d",
		stats.OpenConnections, stats.Idle, stats.InUse, stats.WaitCount)

	assert.LessOrEqual(t, stats.OpenConnections, 5, "Should not exceed max open connections")
}

// TestMySQLPerformance test MySQL performance
func TestMySQLPerformance(t *testing.T) {
	dsn := "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skip("MySQL is not available:", err)
		return
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	ctx := context.Background()

	// Create test table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS perf_test (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			data VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_data (data)
		)
	`)
	require.NoError(t, err)
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS perf_test")

	// Bulk insert performance test
	t.Run("BulkInsert", func(t *testing.T) {
		start := time.Now()

		// Use transaction for bulk insert
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		stmt, err := tx.PrepareContext(ctx, "INSERT INTO perf_test (data) VALUES (?)")
		require.NoError(t, err)

		count := 1000
		for i := 0; i < count; i++ {
			_, err := stmt.ExecContext(ctx, fmt.Sprintf("data_%d", i))
			require.NoError(t, err)
		}

		stmt.Close()
		err = tx.Commit()
		require.NoError(t, err)

		elapsed := time.Since(start)
		opsPerSec := float64(count) / elapsed.Seconds()

		t.Logf("Bulk insert performance: %d rows in %v (%.2f rows/sec)",
			count, elapsed, opsPerSec)

		// Verify performance threshold
		assert.Greater(t, opsPerSec, 500.0, "Should achieve at least 500 inserts/sec")
	})

	// Query performance test
	t.Run("QueryPerformance", func(t *testing.T) {
		start := time.Now()
		queries := 100

		for i := 0; i < queries; i++ {
			var count int
			err := db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM perf_test WHERE data LIKE ?",
				fmt.Sprintf("data_%d%%", i%10),
			).Scan(&count)
			assert.NoError(t, err)
		}

		elapsed := time.Since(start)
		qps := float64(queries) / elapsed.Seconds()

		t.Logf("Query performance: %d queries in %v (%.2f QPS)",
			queries, elapsed, qps)

		// Verify performance threshold
		assert.Greater(t, qps, 100.0, "Should achieve at least 100 QPS")
	})
}
