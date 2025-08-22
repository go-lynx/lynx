package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-lynx/lynx/plugins/swagger" // Import swagger plugin
)

// @title Example API
// @version 1.0
// @description This is a complete Swagger example API
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /api/v1

// Product product model
// @Description Product information
type Product struct {
	// Product ID
	ID int `json:"id" example:"1"`
	// Product name
	// @Required
	// @MinLength 1
	// @MaxLength 100
	Name string `json:"name" example:"iPhone 15"`
	// Product price
	// @Minimum 0
	Price float64 `json:"price" example:"999.99"`
	// Stock quantity
	// @Minimum 0
	Stock int `json:"stock" example:"100"`
	// Product description
	Description string `json:"description,omitempty" example:"Latest smartphone model"`
	// Creation time
	CreatedAt string `json:"created_at" example:"2024-01-01T00:00:00Z"`
}

// PagedResponse paginated response
// @Description Paginated data response structure
type PagedResponse struct {
	// Data list
	Data interface{} `json:"data"`
	// Total count
	Total int `json:"total" example:"100"`
	// Current page
	Page int `json:"page" example:"1"`
	// Page size
	PageSize int `json:"page_size" example:"10"`
}

// ErrorResponse error response
// @Description API error response
type ErrorResponse struct {
	// Error code
	Code int `json:"code" example:"400"`
	// Error message
	Message string `json:"message" example:"Request parameter error"`
	// Detailed information
	Details string `json:"details,omitempty"`
}

// ProductController product controller
type ProductController struct {
	products map[int]*Product
	nextID   int
}

// NewProductController creates a product controller
func NewProductController() *ProductController {
	return &ProductController{
		products: make(map[int]*Product),
		nextID:   1,
	}
}

// ListProducts get product list
// @Summary Get product list
// @Description Get paginated product list with sorting and filtering support
// @Tags Product Management
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param size query int false "Page size" default(10) minimum(1) maximum(100)
// @Param sort query string false "Sort field" Enums(name,price,created_at)
// @Param order query string false "Sort direction" Enums(asc,desc) default(asc)
// @Success 200 {object} PagedResponse{data=[]Product} "Retrieved successfully"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /products [get]
func (c *ProductController) ListProducts(w http.ResponseWriter, r *http.Request) {
	products := make([]*Product, 0, len(c.products))
	for _, p := range c.products {
		products = append(products, p)
	}

	response := PagedResponse{
		Data:     products,
		Total:    len(products),
		Page:     1,
		PageSize: 10,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProduct get product details
// @Summary Get product details
// @Description Get detailed product information by product ID
// @Tags Product Management
// @Accept json
// @Produce json
// @Param id path int true "Product ID" minimum(1)
// @Success 200 {object} Product "Retrieved successfully"
// @Failure 404 {object} ErrorResponse "Product not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /products/{id} [get]
func (c *ProductController) GetProduct(w http.ResponseWriter, r *http.Request) {
	// In actual implementation, ID should be obtained from path parameters
	id := 1

	product, exists := c.products[id]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    404,
			Message: "Product not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// CreateProduct create product
// @Summary Create new product
// @Description Create a new product
// @Tags Product Management
// @Accept json
// @Produce json
// @Param product body Product true "Product information"
// @Success 201 {object} Product "Created successfully"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /products [post]
func (c *ProductController) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    400,
			Message: "Request parameter error",
			Details: err.Error(),
		})
		return
	}

	product.ID = c.nextID
	c.nextID++
	product.CreatedAt = "2024-01-01T00:00:00Z"
	c.products[product.ID] = &product

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

// UpdateProduct update product
// @Summary Update product information
// @Description Update information for specified product
// @Tags Product Management
// @Accept json
// @Produce json
// @Param id path int true "Product ID" minimum(1)
// @Param product body Product true "Product information"
// @Success 200 {object} Product "Updated successfully"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Failure 404 {object} ErrorResponse "Product not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /products/{id} [put]
func (c *ProductController) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	// In actual implementation, ID should be obtained from path parameters
	id := 1

	if _, exists := c.products[id]; !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    404,
			Message: "Product not found",
		})
		return
	}

	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    400,
			Message: "Request parameter error",
			Details: err.Error(),
		})
		return
	}

	product.ID = id
	c.products[id] = &product

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// DeleteProduct delete product
// @Summary Delete product
// @Description Delete specified product
// @Tags Product Management
// @Accept json
// @Produce json
// @Param id path int true "Product ID" minimum(1)
// @Success 204 "Deleted successfully"
// @Failure 404 {object} ErrorResponse "Product not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /products/{id} [delete]
func (c *ProductController) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	// In actual implementation, ID should be obtained from path parameters
	id := 1

	if _, exists := c.products[id]; !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    404,
			Message: "Product not found",
		})
		return
	}

	delete(c.products, id)
	w.WriteHeader(http.StatusNoContent)
}

// SearchProducts search products
// @Summary Search products
// @Description Search products by keywords
// @Tags Product Management
// @Accept json
// @Produce json
// @Param q query string true "Search keywords" minLength(1)
// @Param category query string false "Product category"
// @Param min_price query number false "Minimum price" minimum(0)
// @Param max_price query number false "Maximum price" minimum(0)
// @Success 200 {array} Product "Search successful"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /products/search [get]
func (c *ProductController) SearchProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Code:    400,
			Message: "Search keywords cannot be empty",
		})
		return
	}

	// Simple search implementation
	results := make([]*Product, 0)
	for _, p := range c.products {
		// Actual search logic should be implemented
		results = append(results, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	controller := NewProductController()

	// Add some example data
	controller.products[1] = &Product{
		ID:          1,
		Name:        "iPhone 15",
		Price:       999.99,
		Stock:       100,
		Description: "Latest smartphone model",
		CreatedAt:   "2024-01-01T00:00:00Z",
	}

	// Set up routes
	http.HandleFunc("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			controller.ListProducts(w, r)
		case http.MethodPost:
			controller.CreateProduct(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/v1/products/search", controller.SearchProducts)

	http.HandleFunc("/api/v1/products/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			controller.GetProduct(w, r)
		case http.MethodPut:
			controller.UpdateProduct(w, r)
		case http.MethodDelete:
			controller.DeleteProduct(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Server started on :8081")
	fmt.Println("Swagger documentation access address: http://localhost:8080/swagger")
	fmt.Println("API base path: http://localhost:8081/api/v1")

	log.Fatal(http.ListenAndServe(":8081", nil))
}
