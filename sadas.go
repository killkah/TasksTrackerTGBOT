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
	TaskId int
	Title  string
	Done   bool
}

func AddTask(userID int64, title string, done bool, db *sql.DB) {
	db, err := sql.Open("sqlite", "./todo.db")
	tasks := GetTasks(userID, db)
	maxId := 0
	for _, task := range tasks {
		if task.TaskId > maxId {
			maxId = task.TaskId
		}
	}
	_, err = db.Exec(`
    INSERT INTO tasks (task_id, user_id, title, done) 
    VALUES (
        COALESCE((SELECT MAX(task_id) FROM tasks WHERE user_id = ?), 0) + 1, 
        ?, ?, 0
    )
	`, userID, userID, title)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task added successfully.")
	db.Close()
}

func EditTask(user_id int64, task_id int, done bool, db *sql.DB) {
	_, err := db.Exec(`UPDATE tasks SET done = ? WHERE task_id = ? AND user_id = ?`, done, task_id, user_id)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task updated successfully.")
}

func DeleteTask(user_id int64, task_id int, db *sql.DB) {
	db, err := sql.Open("sqlite", "./todo.db")
	_, err = db.Exec(`DELETE FROM tasks WHERE task_id = ? AND user_id = ?`, task_id, user_id)
	if err != nil {
		ErrorHandler(err)
	}
	_, err = db.Exec("UPDATE tasks SET task_id = task_id - 1 WHERE user_id = ? AND task_id > ?", user_id, task_id)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task deleted successfully.")
	db.Close()
}

func GetTasks(userID int64, db *sql.DB) []Task {

	// 1. Делаем SQL-запрос на получение всех колонок
	rows, err := db.Query("SELECT task_id, user_id, title, done FROM tasks WHERE user_id = ?", userID)
	if err != nil {
		ErrorHandler(err)
	}
	defer rows.Close() // Закрываем результат запроса после завершения функции

	var tasks []Task

	// 2. Проходимся циклом по каждой строчке из БД
	for rows.Next() {
		var t Task
		// Scan считывает значения из БД в поля нашей структуры
		err := rows.Scan(&t.TaskId, &userID, &t.Title, &t.Done)
		if err != nil {
			ErrorHandler(err)
		}
		tasks = append(tasks, t)
	}

	return tasks
}
func TaskExists(task_id int, userID int64, db *sql.DB) (bool, error) {
	db, err := sql.Open("sqlite", "./todo.db")
	if err != nil {
		ErrorHandler(err)
	}
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM tasks WHERE task_id = ? AND user_id = ?)"
	err = db.QueryRow(query, task_id, userID).Scan(&exists)
	db.Close()
	return exists, err

}
func CheckUserConsent(userID int64, db *sql.DB) (bool, error) {
	db, err := sql.Open("sqlite", "./todo.db")
	var accepted bool
	err = db.QueryRow("SELECT accepted_terms FROM users WHERE user_id = ?", userID).Scan(&accepted)
	if err == sql.ErrNoRows {
		return false, nil // Пользователя ещё нет в БД
	}
	db.Close()
	return accepted, err

}
func SaveUserConsent(userID int64, db *sql.DB) error {
	db, err := sql.Open("sqlite", "./todo.db")
	_, err = db.Exec(
		"INSERT INTO users (user_id, accepted_terms, accepted_at) VALUES (?, 1, CURRENT_TIMESTAMP) "+
			"ON CONFLICT(user_id) DO UPDATE SET accepted_terms = 1, accepted_at = CURRENT_TIMESTAMP",
		userID,
	)
	if err != nil {
		ErrorHandler(err)
	}
	db.Close()
	return err
}
func ErrorHandler(err error) {
	fmt.Println("An error occurred:", err)
}
