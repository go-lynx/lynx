package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	_ "github.com/go-lynx/lynx/plugins/nosql/redis"
	_ "github.com/go-lynx/lynx/plugins/polaris"
	_ "github.com/go-lynx/lynx/plugins/sql/mysql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// ProductionApp production-level application example
type ProductionApp struct {
	logger     log.Logger
	httpServer *khttp.Server
	grpcServer *grpc.Server
	db         *sql.DB
	redis      redis.UniversalClient
}

// User user model
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewProductionApp creates a production-level application
func NewProductionApp(logger log.Logger) (*ProductionApp, error) {
	app := &ProductionApp{
		logger: logger,
	}

	// Initialize HTTP server
	httpSrv := app.initHTTPServer()
	app.httpServer = httpSrv

	// Initialize gRPC server
	grpcSrv := app.initGRPCServer()
	app.grpcServer = grpcSrv

	return app, nil
}

// initHTTPServer initializes HTTP server
func (app *ProductionApp) initHTTPServer() *khttp.Server {
	opts := []khttp.ServerOption{
		khttp.Middleware(
			recovery.Recovery(),
		),
		khttp.Filter(
			app.loggingMiddleware(),
			app.rateLimitMiddleware(),
		),
		khttp.Address(":8080"),
		khttp.Timeout(30 * time.Second),
	}

	srv := khttp.NewServer(opts...)

	// Register routes
	router := mux.NewRouter()

	// Health checks
	router.HandleFunc("/health", app.healthCheck).Methods("GET")
	router.HandleFunc("/ready", app.readinessCheck).Methods("GET")

	// API routes
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// User-related routes
	apiRouter.HandleFunc("/users", app.listUsers).Methods("GET")
	apiRouter.HandleFunc("/users", app.createUser).Methods("POST")
	apiRouter.HandleFunc("/users/{id}", app.getUser).Methods("GET")
	apiRouter.HandleFunc("/users/{id}", app.updateUser).Methods("PUT")
	apiRouter.HandleFunc("/users/{id}", app.deleteUser).Methods("DELETE")

	// Cache demo routes
	apiRouter.HandleFunc("/cache/get/{key}", app.getCache).Methods("GET")
	apiRouter.HandleFunc("/cache/set/{key}", app.setCache).Methods("POST")
	apiRouter.HandleFunc("/cache/del/{key}", app.delCache).Methods("DELETE")

	srv.Handle("/", router)

	return srv
}

// initGRPCServer initializes gRPC server
func (app *ProductionApp) initGRPCServer() *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
		grpc.Address(":9090"),
		grpc.Timeout(30 * time.Second),
	}

	srv := grpc.NewServer(opts...)
	// gRPC services can be registered here

	return srv
}

// Middleware

// loggingMiddleware logging middleware
func (app *ProductionApp) loggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap ResponseWriter to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			app.logger.Log(log.LevelInfo,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", duration.String(),
				"ip", r.RemoteAddr,
			)
		})
	}
}

// rateLimitMiddleware rate limiting middleware
func (app *ProductionApp) rateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redis-based rate limiting can be implemented here
			// Use the RateLimiter created above
			next.ServeHTTP(w, r)
		})
	}
}

// API handlers

// healthCheck health check
func (app *ProductionApp) healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// readinessCheck readiness check
func (app *ProductionApp) readinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connection
	dbHealthy := true
	if app.db != nil {
		if err := app.db.PingContext(ctx); err != nil {
			dbHealthy = false
			app.logger.Log(log.LevelError, "msg", "Database health check failed", "error", err)
		}
	}

	// Check Redis connection
	redisHealthy := true
	if app.redis != nil {
		if err := app.redis.Ping(ctx).Err(); err != nil {
			redisHealthy = false
			app.logger.Log(log.LevelError, "msg", "Redis health check failed", "error", err)
		}
	}

	response := map[string]interface{}{
		"ready":    dbHealthy && redisHealthy,
		"database": dbHealthy,
		"redis":    redisHealthy,
		"time":     time.Now().UTC(),
	}

	status := http.StatusOK
	if !response["ready"].(bool) {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// listUsers get user list
func (app *ProductionApp) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get from cache
	cacheKey := "users:list"
	if app.redis != nil {
		cached, err := app.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))
			return
		}
	}

	// Query from database
	users := []User{}
	if app.db != nil {
		rows, err := app.db.QueryContext(ctx, "SELECT id, username, email, created_at, updated_at FROM users LIMIT 100")
		if err != nil {
			app.handleError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var user User
			if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt); err != nil {
				app.handleError(w, err, http.StatusInternalServerError)
				return
			}
			users = append(users, user)
		}
	}

	// Serialize response
	data, err := json.Marshal(users)
	if err != nil {
		app.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Cache result
	if app.redis != nil {
		app.redis.Set(ctx, cacheKey, string(data), 5*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// createUser create user
func (app *ProductionApp) createUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		app.handleError(w, err, http.StatusBadRequest)
		return
	}

	// Validate input
	if user.Username == "" || user.Email == "" {
		app.handleError(w, fmt.Errorf("username and email are required"), http.StatusBadRequest)
		return
	}

	// Insert into database
	if app.db != nil {
		result, err := app.db.ExecContext(ctx,
			"INSERT INTO users (username, email, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
			user.Username, user.Email,
		)
		if err != nil {
			app.handleError(w, err, http.StatusInternalServerError)
			return
		}

		id, _ := result.LastInsertId()
		user.ID = id
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()
	}

	// Clear cache
	if app.redis != nil {
		app.redis.Del(ctx, "users:list")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// getUser get single user
func (app *ProductionApp) getUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Try to get from cache
	cacheKey := fmt.Sprintf("user:%s", id)
	if app.redis != nil {
		cached, err := app.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))
			return
		}
	}

	// Query from database
	var user User
	if app.db != nil {
		row := app.db.QueryRowContext(ctx,
			"SELECT id, username, email, created_at, updated_at FROM users WHERE id = ?",
			id,
		)
		if err := row.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt); err != nil {
			if err == sql.ErrNoRows {
				app.handleError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			} else {
				app.handleError(w, err, http.StatusInternalServerError)
			}
			return
		}
	}

	// Serialize response
	data, err := json.Marshal(user)
	if err != nil {
		app.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Cache result
	if app.redis != nil {
		app.redis.Set(ctx, cacheKey, string(data), 10*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// updateUser update user
func (app *ProductionApp) updateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		app.handleError(w, err, http.StatusBadRequest)
		return
	}

	// Update database
	if app.db != nil {
		result, err := app.db.ExecContext(ctx,
			"UPDATE users SET username = ?, email = ?, updated_at = NOW() WHERE id = ?",
			user.Username, user.Email, id,
		)
		if err != nil {
			app.handleError(w, err, http.StatusInternalServerError)
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			app.handleError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			return
		}
	}

	// Clear cache
	if app.redis != nil {
		app.redis.Del(ctx, fmt.Sprintf("user:%s", id), "users:list")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// deleteUser delete user
func (app *ProductionApp) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Delete from database
	if app.db != nil {
		result, err := app.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
		if err != nil {
			app.handleError(w, err, http.StatusInternalServerError)
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			app.handleError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			return
		}
	}

	// Clear cache
	if app.redis != nil {
		app.redis.Del(ctx, fmt.Sprintf("user:%s", id), "users:list")
	}

	w.WriteHeader(http.StatusNoContent)
}

// Cache operation demo

// getCache get cache
func (app *ProductionApp) getCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	key := vars["key"]

	if app.redis == nil {
		app.handleError(w, fmt.Errorf("redis not available"), http.StatusServiceUnavailable)
		return
	}

	val, err := app.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			app.handleError(w, fmt.Errorf("key not found"), http.StatusNotFound)
		} else {
			app.handleError(w, err, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"key":   key,
		"value": val,
	})
}

// setCache set cache
func (app *ProductionApp) setCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	key := vars["key"]

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		app.handleError(w, err, http.StatusBadRequest)
		return
	}

	value, ok := payload["value"].(string)
	if !ok {
		app.handleError(w, fmt.Errorf("value must be a string"), http.StatusBadRequest)
		return
	}

	ttl := 5 * time.Minute
	if ttlSeconds, ok := payload["ttl"].(float64); ok {
		ttl = time.Duration(ttlSeconds) * time.Second
	}

	if app.redis == nil {
		app.handleError(w, fmt.Errorf("redis not available"), http.StatusServiceUnavailable)
		return
	}

	if err := app.redis.Set(ctx, key, value, ttl).Err(); err != nil {
		app.handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   key,
		"value": value,
		"ttl":   ttl.Seconds(),
	})
}

// delCache delete cache
func (app *ProductionApp) delCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	key := vars["key"]

	if app.redis == nil {
		app.handleError(w, fmt.Errorf("redis not available"), http.StatusServiceUnavailable)
		return
	}

	if err := app.redis.Del(ctx, key).Err(); err != nil {
		app.handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

// handleError handle error response
func (app *ProductionApp) handleError(w http.ResponseWriter, err error, status int) {
	app.logger.Log(log.LevelError, "error", err.Error(), "status", status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

// responseWriter wraps ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// main main function
func main() {
	// Create logger
	logger := log.NewStdLogger(os.Stdout)
	log := log.NewHelper(logger)

	// Create application
	app, err := NewProductionApp(logger)
	if err != nil {
		log.Fatal(err)
	}

	// Create Kratos application
	kratosApp := kratos.New(
		kratos.ID("production-app"),
		kratos.Name("Production Application"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(
			app.httpServer,
			app.grpcServer,
		),
	)

	// Start application
	go func() {
		if err := kratosApp.Run(); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	if err := kratosApp.Stop(); err != nil {
		log.Error(err)
	}

	// Close database connection
	if app.db != nil {
		app.db.Close()
	}

	// Close Redis connection
	if app.redis != nil {
		app.redis.Close()
	}

	log.Info("Server exited")
}
