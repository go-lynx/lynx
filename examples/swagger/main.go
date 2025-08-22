package main

import (
	"fmt"
	"net/http"

	"github.com/go-lynx/lynx/app/log"
	_ "github.com/go-lynx/lynx/plugins/swagger" // Import swagger plugin
)

// UserRequest user request structure
// @Description User creation request
type UserRequest struct {
	// Username
	// @Required
	// @MinLength 3
	// @MaxLength 20
	Name string `json:"name" binding:"required"`

	// Email address
	// @Format email
	Email string `json:"email" binding:"required,email"`

	// Age
	// @Minimum 1
	// @Maximum 120
	Age int `json:"age" binding:"min=1,max=120"`
}

// UserResponse user response structure
// @Description User information response
type UserResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Age       int    `json:"age"`
	CreatedAt string `json:"created_at"`
}

// ErrorResponse error response
// @Description API error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// UserController user controller
type UserController struct{}

// CreateUser create user
// @Summary Create new user
// @Description Create a new user account
// @Tags User Management
// @Accept json
// @Produce json
// @Param user body UserRequest true "User information"
// @Success 201 {object} UserResponse "Created successfully"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/users [post]
func (c *UserController) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Implement user creation logic
	log.Info("Creating user...")
}

// GetUser get user information
// @Summary Get user details
// @Description Get user detailed information by user ID
// @Tags User Management
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} UserResponse "Retrieved successfully"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/users/{id} [get]
func (c *UserController) GetUser(w http.ResponseWriter, r *http.Request) {
	// Implement get user logic
	log.Info("Getting user...")
}

// UpdateUser update user information
// @Summary Update user information
// @Description Update information for the specified user
// @Tags User Management
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param user body UserRequest true "User information"
// @Success 200 {object} UserResponse "Updated successfully"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/users/{id} [put]
func (c *UserController) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Implement update user logic
	log.Info("Updating user...")
}

// DeleteUser delete user
// @Summary Delete user
// @Description Delete the specified user account
// @Tags User Management
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 204 "Deleted successfully"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/users/{id} [delete]
func (c *UserController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Implement delete user logic
	log.Info("Deleting user...")
}

// ListUsers get user list
// @Summary Get user list
// @Description Get paginated user list
// @Tags User Management
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param sort query string false "Sort field" Enums(name,created_at)
// @Success 200 {array} UserResponse "Retrieved successfully"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/users [get]
func (c *UserController) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Implement get user list logic
	log.Info("Listing users...")
}

func main() {
	// Create HTTP server to demonstrate API
	http.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		controller := &UserController{}
		switch r.Method {
		case http.MethodGet:
			controller.ListUsers(w, r)
		case http.MethodPost:
			controller.CreateUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		controller := &UserController{}
		switch r.Method {
		case http.MethodGet:
			controller.GetUser(w, r)
		case http.MethodPut:
			controller.UpdateUser(w, r)
		case http.MethodDelete:
			controller.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Info("Starting server on :8081...")
	log.Info("Visit http://localhost:8080/swagger for API documentation")

	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}
