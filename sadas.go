package main

import (
	"database/sql"
	"fmt"

	//"net/http"
	//"net/url"

	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	tasks := GetTasks(userID, db)
	maxId := 0
	for _, task := range tasks {
		if task.TaskId > maxId {
			maxId = task.TaskId
		}
	}
	_, err := db.Exec(`
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
}

func EditTask(user_id int64, task_id int, done bool, db *sql.DB) {
	_, err := db.Exec(`UPDATE tasks SET done = ? WHERE task_id = ? AND user_id = ?`, done, task_id, user_id)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task updated successfully.")
}

func DeleteTask(user_id int64, task_id int, db *sql.DB) {
	_, err := db.Exec(`DELETE FROM tasks WHERE task_id = ? AND user_id = ?`, task_id, user_id)
	if err != nil {
		ErrorHandler(err)
	}
	_, err = db.Exec("UPDATE tasks SET task_id = task_id - 1 WHERE user_id = ? AND task_id > ?", user_id, task_id)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Task deleted successfully.")
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
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM tasks WHERE task_id = ? AND user_id = ?)"
	err := db.QueryRow(query, task_id, userID).Scan(&exists)
	if err != nil {
		ErrorHandler(err)
	}
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
	if err != nil {
		ErrorHandler(err)
	}
	return err
}
func PrintTasks(tasks []Task, chatID int64, bot *tgbotapi.BotAPI) {
	if len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📋 Список задач пуст.")
		bot.Send(msg)
		return
	}
	var response strings.Builder
	response.WriteString("📋 Ваши задачи:\n\n")
	for _, task := range tasks {
		status := "❌"
		if task.Done {
			status = "✅"
		}

		response.WriteString(fmt.Sprintf("%d %s - %s\n", task.TaskId, task.Title, status))
	}
	msg := tgbotapi.NewMessage(chatID, response.String())
	bot.Send(msg)
}
func ErrorHandler(err error) {
	fmt.Println("An error occurred:", err)
}
