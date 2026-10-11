package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var mainKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("📋 Мои задачи"),
		tgbotapi.NewKeyboardButton("➕ Добавить задачу"),
	),
)

func HandleStart(bot *tgbotapi.BotAPI, chatID int64, userID int64, db *sql.DB) {

	accepted, err := CheckUserConsent(userID, db)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка проверки профиля."))
	}

	if !accepted {
		msgText := "👋 **Добро пожаловать в Таск-менеджер!**\n\n" +
			"Для использования бота необходимо ознакомиться и согласиться с " +
			"[Политикой конфиденциальности](https://github.com/killkah/TasksTrackerTGBOT/blob/main/Privacy.md) " +
			"и [Согласием на обработку данных](https://github.com/killkah/TasksTrackerTGBOT/blob/main/Consent.md).\n\n" +
			"Нажимая кнопку «Принять», вы даёте согласие на обработку вашего Telegram ID и создаваемых задач."

		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Принять условия", "accept_terms"),
			),
		)
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	}
	welcomeMsg := tgbotapi.NewMessage(chatID, "👋 Привет! Я твой таск-менеджер.\n\n"+
		"Команды:\n"+
		"📋 /tasks — Посмотреть все задачи\n"+
		"➕ /add Текст — Добавить новую задачу\n"+
		"❌ /delete ID — Удалить задачу по ID")
	bot.Send(welcomeMsg)

}

func HandlePrintTasks(bot *tgbotapi.BotAPI, chatID int64, userID int64, db *sql.DB) {
	tasks := GetTasks(userID, db)
	if len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас нет задач.")
		bot.Send(msg)
	}
	PrintTasks(tasks, chatID, bot)
}

func HandleAddTask(text string, bot *tgbotapi.BotAPI, chatID int64, userID int64, db *sql.DB) {
	title := strings.TrimPrefix(text, "/add ")
	title = strings.TrimSpace(title)
	if title == "" {
		msg := tgbotapi.NewMessage(chatID, "Укажите название задачи после команды /add.")
		bot.Send(msg)
	}
	if utf8.RuneCountInString(title) > 50 {
		msg := tgbotapi.NewMessage(chatID, "Слишком длинное название(куда тебе 50).")
		bot.Send(msg)
	}
	AddTask(userID, title, false, db)
	msg := tgbotapi.NewMessage(chatID, "Задача добавлена: "+title)
	bot.Send(msg)
}

func HandleEditTask(text string, bot *tgbotapi.BotAPI, chatID int64, userID int64, db *sql.DB) {
	idStr := strings.TrimPrefix(text, "/edit ")
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		msg := tgbotapi.NewMessage(chatID, "Укажите название задачи которую хотите изменить после /edit.")
		bot.Send(msg)

	}
	id, err := strconv.Atoi(idStr)
	exists, err := TaskExists(id, userID, db)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при проверке базы данных."))

	}

	if !exists {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Задачи с ID #%d у вас не существует!", id)))

	}
	rows, err := db.Query("SELECT task_id, title, done FROM tasks WHERE task_id = ? AND user_id = ?", id, userID)
	if err != nil {
		ErrorHandler(err)
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(&t.TaskId, &t.Title, &t.Done)
		if err != nil {
			ErrorHandler(err)
			continue
		}
		tasks = append(tasks, t)
	}
	if tasks[0].Done == false {
		tasks[0].Done = true
	} else {
		tasks[0].Done = false
	}
	EditTask(userID, tasks[0].TaskId, tasks[0].Done, db)
	msg := tgbotapi.NewMessage(chatID, "Задача успешно обновлена")
	bot.Send(msg)
}

func HandleDeleteTask(text string, bot *tgbotapi.BotAPI, chatID int64, userID int64, db *sql.DB) {
	idStr := strings.TrimPrefix(text, "/delete ")
	idStr = strings.TrimSpace(idStr)
	id, err := strconv.Atoi(idStr)
	exists, err := TaskExists(id, userID, db)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при проверке базы данных."))

	}

	if !exists {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Задачи с ID #%d у вас не существует!", id)))

	}
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Укажите корректный ID задачи после команды /delete.")
		bot.Send(msg)

	}
	DeleteTask(userID, id, db)
	msg := tgbotapi.NewMessage(chatID, "Задача удалена.")
	bot.Send(msg)
}

func HandlePrivacy(bot *tgbotapi.BotAPI, chatID int64) {
	privacyMsg := "🔒 *Политика конфиденциальности*\n\n" +
		"Мы храним только ваш Telegram ID и тексты созданных задач для работы списка дел.\n" +
		"Данные не передаются третьим лицам.\n\n" +
		"Подробные условия: https://github.com/killkah/TasksTrackerTGBOT/blob/main/Privacy.md"

	msg := tgbotapi.NewMessage(chatID, privacyMsg)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}
