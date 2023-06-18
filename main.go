package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	_ "github.com/denisenkom/go-mssqldb" // MS SQL Server driver
)

func getConnection() (*sql.DB, error) {
	server := "CODELINE002"
	port := 1433
	database := "CodelineChallenge1"
	user := "sa"
	password := "root"

	connectionString := fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s",
		server, port, database, user, password)

	db, err := sql.Open("sqlserver", connectionString)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func createTables() error {
	db, err := getConnection()
	if err != nil {
		return err
	}
	defer db.Close()

	// Check if the user_table exists
	if tableExists(db, "user_table") {
		fmt.Println("user_table already exists")
	} else {
		// Create the user_table
		createUserTable := `
		CREATE TABLE user_table (
			id INT IDENTITY(1, 1) PRIMARY KEY,
			name VARCHAR(255),
			last_login_date DATETIME
		)
		`

		_, err := db.Exec(createUserTable)
		if err != nil {
			return err
		}
		fmt.Println("user_table created")
	}

	// Check if the measurement_result_table exists
	if tableExists(db, "measurement_result_table") {
		fmt.Println("measurement_result_table already exists")
	} else {
		// Create the measurement_result_table
		createMeasurementResultTable := `
		CREATE TABLE measurement_result_table (
			measurement_result_id INT IDENTITY(1,1) PRIMARY KEY,
			measurement_value VARCHAR(255),
			result_value VARCHAR(255)
		)
		`

		_, err := db.Exec(createMeasurementResultTable)
		if err != nil {
			return err
		}
		fmt.Println("measurement_result_table created")
	}

	// Check if the user_activity_table exists
	if tableExists(db, "user_activity_table") {
		fmt.Println("user_activity_table already exists")
	} else {
		// Create the user_activity_table
		createUserActivityTable := `
		CREATE VIEW third_table_view AS
		SELECT u.last_login_date, u.id AS user_id, u.name, m.measurement_value, m.measurement_result_id
		FROM user_table u
		CROSS JOIN measurement_result_table m;								
		`

		_, err := db.Exec(createUserActivityTable)
		if err != nil {
			return err
		}
		fmt.Println("user_activity_table created")
	}

	return nil
}

func tableExists(db *sql.DB, tableName string) bool {
	query := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = '%s'", tableName)
	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		log.Println("Error checking table existence:", err)
		return false
	}
	return count > 0
}

func main() {
	err := createTables()
	if err != nil {
		fmt.Println("Error creating tables:", err)
	} else {
		fmt.Println("Tables created successfully")
	}

	http.HandleFunc("/user", userHandler)
	http.HandleFunc("/convert-measurements", convertMeasurementsHandler)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs) // Serve static files
	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	err := r.ParseForm()
	if err != nil {
		log.Println("Error parsing form data:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	username := r.FormValue("username")
	loginDate := time.Now()
	err = storeUserLogin(username, loginDate)
	if err != nil {
		log.Println("Error storing user login:", err)
		// Return an error response if desired
	}
}

func convertMeasurementsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	err := r.ParseForm()
	if err != nil {
		log.Println("Error parsing form data:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// username := r.FormValue("username")
	// loginDate := time.Now()
	// err = storeUserLogin(username, loginDate)
	// if err != nil {
	// 	log.Println("Error storing user login:", err)
	// 	// Return an error response if desired
	// }

	measurements := r.Form.Get("convert-measurements")
	result := convertMeasurements(measurements)
	response := struct {
		Result []int `json:"result"`
	}{
		Result: result,
	}

	err = storeMeasurementResult(measurements, result)
	if err != nil {
		log.Println("Error storing measurement result:", err)
		// Return an error response if desired
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func storeUserLogin(username string, loginDate time.Time) error {
	db, err := getConnection()
	if err != nil {
		return err
	}
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO user_table (name, last_login_date) VALUES (@name, @last_login_date)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	fmt.Println("Storing user login:")
	fmt.Println("Username:", username)
	fmt.Println("Login Date:", loginDate)

	_, err = stmt.Exec(sql.Named("name", username), sql.Named("last_login_date", loginDate))
	if err != nil {
		return err
	}

	return nil
}

func storeMeasurementResult(measurementValue string, result []int) error {
	db, err := getConnection()
	if err != nil {
		return err
	}
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO measurement_result_table (measurement_value, result_value) VALUES (@measurement_value, @result_value)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	resultValue := resultToString(result)

	_, err = stmt.Exec(sql.Named("measurement_value", measurementValue), sql.Named("result_value", resultValue))
	if err != nil {
		return err
	}

	return nil
}

func resultToString(result []int) string {
	str := ""
	for i, value := range result {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf("%d", value)
	}
	return str
}

func convertMeasurements(str string) []int {
	collectedValues := make([]int, 0)
	if isValidSeq(str) {
		isNewNumber := true
		isValAfterZ := false
		totalZValues := 0
		roundLength := 0
		roundItr := 0
		roundTotal := 0
		charVal := 0

		for i := 0; i < len(str); i++ {
			char := str[i]
			charVal = int(str[i]) - 96
			if char == '_' {
				charVal = 0
				if isValAfterZ {
					charVal += totalZValues
					totalZValues = 0
					isValAfterZ = false
				}
			} else if char == 'z' {
				isValAfterZ = true
				totalZValues += charVal
				continue
			} else {
				charVal = int(char) - 96
				if isValAfterZ {
					charVal += totalZValues
					totalZValues = 0
					isValAfterZ = false
				}
			}

			if !isNewNumber {
				roundTotal += charVal
				roundItr++
			} else {
				roundLength = charVal
				roundTotal = 0
				roundItr = 0
				isNewNumber = false
			}

			if roundItr == roundLength {
				collectedValues = append(collectedValues, roundTotal)
				isNewNumber = true
			}

			if i == len(str)-1 && roundItr != roundLength {
				collectedValues = append(collectedValues, 0)
			}
		}
	}
	return collectedValues
}

func isValidSeq(str string) bool {
	pattern := "^[a-z_]+$"
	match, _ := regexp.MatchString(pattern, str)
	return match
}
