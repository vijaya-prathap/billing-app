package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// MySQL connection string
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	dsn := dbUser + ":" + dbPassword + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName

	// Open database connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("failed to open MySQL connection:", err)
	}
	defer db.Close()

	// Verify MySQL connection
	if err := db.Ping(); err != nil {
		log.Fatal("failed to connect to MySQL:", err)
	}

	log.Println("MySQL connected successfully!")

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Get all customers
	router.GET("/customers", func(c *gin.Context) {
		var customers []gin.H

		rows, err := db.Query("SELECT id, name, email, created_at FROM customers")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch customers",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var name string
			var email string
			var createdAt string

			if err := rows.Scan(&id, &name, &email, &createdAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to read customer data",
				})
				return
			}

			customers = append(customers, gin.H{
				"id":         id,
				"name":       name,
				"email":      email,
				"created_at": createdAt,
			})
		}

		c.JSON(http.StatusOK, customers)
	})

	log.Println("Server running on port 8080")

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
