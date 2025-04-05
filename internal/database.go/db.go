package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var DB *sql.DB

func InitDB() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatal("Database connection is not active:", err)
	}

	fmt.Println("Database connected successfully!")
}

func GetSqlQueryRow(query string, args ...interface{}) (map[string]interface{}, error) {
	// row := DB.QueryRow(query, args...)

	// Get column names using a prepared statement
	stmt, err := DB.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Fetch single row
	if rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		result := make(map[string]interface{})
		for i, col := range columns {
			result[col] = values[i]

			// Check if the value is of type []byte (typically used for BLOBs or encoded data)
			if b, ok := values[i].([]byte); ok {
				result[col] = string(b)
			}
		}

		return result, nil
	}

	return nil, fmt.Errorf("no rows found")
}

func GetSqlQueryRows(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var result []map[string]interface{}
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]

			// Check if the value is of type []byte (typically used for BLOBs or encoded data)
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			}
		}
		result = append(result, row)
	}

	return result, nil
}

func SendSqlStatement(query string, args ...interface{}) error {
	_, err := DB.Exec(query, args...)
	return err
}
