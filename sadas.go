package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // Импорт Pure-Go драйвера
)

// Task — описывает одну задачу
type Task struct {
	ID     int
	UserId int64
	Title  string
	Done   bool
}

func AddTask(userID int64, title string, done bool, db *sql.DB) {
	_, err := db.Exec(`INSERT INTO tasks (user_id ,title, done) VALUES (?, ?, 0)`, userID, title)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task added successfully.")
}

func EditTask(id int, done bool, db *sql.DB) {
	_, err := db.Exec(`UPDATE tasks SET done = ? WHERE id = ?`, done, id)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task updated successfully.")
}

func DeleteTask(id int, userID int64, db *sql.DB) {
	_, err := db.Exec(`DELETE FROM tasks WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task deleted successfully.")
}

func GetTasks(userID int64, db *sql.DB) []Task {
	// 1. Делаем SQL-запрос на получение всех колонок
	rows, err := db.Query("SELECT id, user_id, title, done FROM tasks WHERE user_id = ?", userID)
	if err != nil {
		ErrorHandler(err)
	}
	defer rows.Close() // Закрываем результат запроса после завершения функции

	var tasks []Task

	// 2. Проходимся циклом по каждой строчке из БД
	for rows.Next() {
		var t Task
		// Scan считывает значения из БД в поля нашей структуры
		err := rows.Scan(&t.ID, &userID, &t.Title, &t.Done)
		if err != nil {
			ErrorHandler(err)
		}
		tasks = append(tasks, t)
	}

	return tasks
}
func TaskExists(id int, userID int64, db *sql.DB) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM tasks WHERE id = ? AND user_id = ?)"
	err := db.QueryRow(query, id, userID).Scan(&exists)
	return exists, err
}
func CheckUserConsent(userID int64, db *sql.DB) (bool, error) {
	var accepted bool
	err := db.QueryRow("SELECT accepted_terms FROM users WHERE user_id = ?", userID).Scan(&accepted)
	if err == sql.ErrNoRows {
		return false, nil // Пользователя ещё нет в БД
	}
	return accepted, err
}
func SaveUserConsent(userID int64, db *sql.DB) error {
	_, err := db.Exec(
		"INSERT INTO users (user_id, accepted_terms, accepted_at) VALUES (?, 1, CURRENT_TIMESTAMP) "+
			"ON CONFLICT(user_id) DO UPDATE SET accepted_terms = 1, accepted_at = CURRENT_TIMESTAMP",
		userID,
	)
	return err
}
func ErrorHandler(err error) {
	fmt.Println("An error occurred:", err)
}
