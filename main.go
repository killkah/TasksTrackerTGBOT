package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite" // Импорт Pure-Go драйвера
)

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

		response.WriteString(fmt.Sprintf("%d %s - %s\n", task.ID, task.Title, status))
	}
	msg := tgbotapi.NewMessage(chatID, response.String())
	bot.Send(msg)
}
func main() {
	proxyURL, err := url.Parse("socks5://127.0.0.1:1080")
	if err != nil {
		log.Fatalf("Ошибка парсинга URL прокси: %v", err)
	}
	// 2. Создаем HTTP-клиент, работающий через прокси
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	db, err := sql.Open("sqlite", "./todo.db")
	if err != nil {
		ErrorHandler(err)
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		done BOOLEAN NOT NULL DEFAULT 0
	);`
	createTableUsers := `
	CREATE TABLE IF NOT EXISTS users (
    user_id INTEGER PRIMARY KEY,
    accepted_terms BOOLEAN DEFAULT 0,
    accepted_at DATETIME
	);`

	_, err = db.Exec(createTableQuery)
	_, err = db.Exec(createTableUsers)
	if err != nil {
		ErrorHandler(err)
	}
	fmt.Println("Таблица tasks успешно создана или уже существует.")

	if err != nil {
		ErrorHandler(err)
	}

	err = godotenv.Load()
	if err != nil {
		log.Println("Предупреждение: .env файл не найден, пробуем взять из системы")
	}
	// 3. Инициализируем бота с использованием прокси-клиента
	token := os.Getenv("TELEGRAM_APITOKEN")
	if token == "" {
		// Если токен не задан в переменных окружения, укажите его прямо тут:
		// token = "ТВОЙ_ТОКЕН_БОТА"
		log.Fatal("TELEGRAM_APITOKEN не установлен")
	} // Подключение к Telegram Bot API
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		log.Fatalf("Ошибка авторизации бота: %v", err)
	}
	bot.Debug = true
	log.Printf("Авторизован как бот: %s", bot.Self.UserName)

	// 3. Настраиваем получение обновлений (сообщений)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// 4. Бесконечный цикл обработки входящих сообщений
	for update := range updates {
		// 1. ОБРАБОТКА НАЖАТИЯ INLINE-КНОПОК (CallbackQuery)
		if update.CallbackQuery != nil {
			callback := update.CallbackQuery
			chatID := callback.Message.Chat.ID
			userID := callback.From.ID

			// ОБЯЗАТЕЛЬНО: Отвечаем Telegram, чтобы кнопка перестала крутиться
			callbackAnswer := tgbotapi.NewCallback(callback.ID, "Условия приняты!")
			if _, err := bot.Request(callbackAnswer); err != nil {
				log.Println("Ошибка отправки CallbackAnswer:", err)
			}

			// Проверяем, какая именно кнопка была нажата
			if callback.Data == "accept_terms" {
				// Сохраняем факт согласия в БД
				err := SaveUserConsent(userID, db)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка сохранения согласия в БД."))
					continue
				}

				// (Опционально) Удаляем или редактируем сообщение с кнопкой, чтобы нельзя было нажать повторно
				editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, "✅ Вы успешно приняли условия использования!")
				bot.Send(editMsg)

				// Отправляем приветственное сообщение
				welcomeMsg := tgbotapi.NewMessage(chatID, "👋 Привет! Я твой таск-менеджер.\n\n"+
					"Команды:\n"+
					"📋 /tasks — Посмотреть все задачи\n"+
					"➕ /add Текст — Добавить новую задачу\n"+
					"❌ /delete ID — Удалить задачу по ID")
				bot.Send(welcomeMsg)
			}
			continue // Переходим к следующему update
		}

		// 2. ИГНОРИРУЕМ ВСЕ СООБЩЕНИЯ, ЕСЛИ ЭТО НЕ ТЕКСТ
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID
		text := update.Message.Text

		// 3. БЛОКИРОВКА ДОСТУПА: Проверяем согласие перед вызовом любых команд
		accepted, err := CheckUserConsent(userID, db)
		if err != nil {
			log.Println("Ошибка проверки согласия:", err)
		}

		if !accepted && text != "/start" {
			msg := tgbotapi.NewMessage(chatID, "⚠️ Чтобы пользоваться ботом, нужно принять условия. Напишите /start")
			bot.Send(msg)
			continue
		}
		// Обработка команд
		switch {
		// Команда /start
		case text == "/start":
			// Проверяем в БД, давал ли пользователь согласие ранее
			accepted, err := CheckUserConsent(userID, db)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Ошибка проверки профиля."))
				continue
			}

			if !accepted {
				msgText := "👋 **Добро пожаловать в Таск-менеджер!**\n\n" +
					"Для использования бота необходимо ознакомиться и согласиться с " +
					"[Политикой конфиденциальности](https://github.com/killkah/TasksTrackerTGBOT/blob/main/Privacy.md) " +
					"и [Согласием на обработку данных](https://github.com/killkah/TasksTrackerTGBOT/blob/main/Consent.md).\n\n" +
					"Нажимая кнопку «Принять», вы даёте согласие на обработку вашего Telegram ID и создаваемых задач."

				msg := tgbotapi.NewMessage(chatID, msgText)
				msg.ParseMode = "Markdown"

				// Создаём Inline-кнопку для акцепта
				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Принять условия", "accept_terms"),
					),
				)
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}

			// Если уже давал согласие — выводим стандартную справку
			msg := tgbotapi.NewMessage(chatID, "👋 Привет! Я твой таск-менеджер.\n\n"+
				"Команды:\n"+
				"📋 /tasks — Посмотреть все задачи\n"+
				"➕ /add Текст — Добавить новую задачу\n"+
				"❌ /delete ID — Удалить задачу по ID")
			bot.Send(msg)
		// Команда /tasks
		case strings.HasPrefix(text, "/tasks"):
			tasks := GetTasks(userID, db)
			if len(tasks) == 0 {
				msg := tgbotapi.NewMessage(chatID, "У вас нет задач.")
				bot.Send(msg)
			}
			PrintTasks(tasks, chatID, bot)
		//  Команда /add
		case strings.HasPrefix(text, "/add "):
			title := strings.TrimPrefix(text, "/add ")
			title = strings.TrimSpace(title)
			if title == "" {
				msg := tgbotapi.NewMessage(chatID, "Укажите название задачи после команды /add.")
				bot.Send(msg)
				continue
			}
			if len(title) > 50 {
				msg := tgbotapi.NewMessage(chatID, "Слишком длинное название(куда тебе 50).")
				bot.Send(msg)
				continue
			}
			AddTask(userID, title, false, db)
			msg := tgbotapi.NewMessage(chatID, "Задача добавлена: "+title)
			bot.Send(msg)
		// Команда /edit
		case strings.HasPrefix(text, "/edit "):

			idStr := strings.TrimPrefix(text, "/edit ")
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				msg := tgbotapi.NewMessage(chatID, "Укажите название задачи которую хотите изменить после /edit.")
				bot.Send(msg)
				continue

			}
			id, err := strconv.Atoi(idStr)
			// 1. Проверяем существование записи
			exists, err := TaskExists(id, userID, db)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при проверке базы данных."))
				continue
			}

			if !exists {
				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Задачи с ID #%d у вас не существует!", id)))
				continue
			}
			rows, err := db.Query("SELECT id, title, done FROM tasks WHERE id = ?", id)
			if err != nil {
				ErrorHandler(err)
			}
			defer rows.Close() // Обязательно закрываем rows после работы
			var tasks []Task
			for rows.Next() {
				var t Task
				err := rows.Scan(&t.ID, &t.Title, &t.Done)
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
			EditTask(tasks[0].ID, tasks[0].Done, db)
			msg := tgbotapi.NewMessage(chatID, "Задача успешно обновлена")
			bot.Send(msg)
		// Команда /delete
		case strings.HasPrefix(text, "/delete "):
			idStr := strings.TrimPrefix(text, "/delete ")
			idStr = strings.TrimSpace(idStr)
			id, err := strconv.Atoi(idStr)
			// 1. Проверяем существование записи
			exists, err := TaskExists(id, userID, db)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при проверке базы данных."))
				continue
			}

			if !exists {
				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Задачи с ID #%d у вас не существует!", id)))
				continue
			}
			if err != nil {
				msg := tgbotapi.NewMessage(chatID, "Укажите корректный ID задачи после команды /delete.")
				bot.Send(msg)
				continue
			}
			DeleteTask(id, userID, db)
			msg := tgbotapi.NewMessage(chatID, "Задача удалена.")
			bot.Send(msg)
		case text == "/privacy":
			privacyMsg := "🔒 *Политика конфиденциальности*\n\n" +
				"Мы храним только ваш Telegram ID и тексты созданных задач для работы списка дел.\n" +
				"Данные не передаются третьим лицам.\n\n" +
				"Подробные условия: https://github.com/killkah/TasksTrackerTGBOT/blob/main/Privacy.md"

			msg := tgbotapi.NewMessage(chatID, privacyMsg)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}
	}
	// for {

	// 	fmt.Println("Выбирете действие: 1 - Посмореть все задачи, 2 - Добавить задачу, 3 - Редактировать задачу, 4 - Удалить задачу, 5 - Выход: ")
	// 	input, err := reader.ReadString('\n')
	// 	if err != nil {
	// 		ErrorHandler(err)
	// 	}

	// 	action := strings.TrimSpace(input)
	// 	switch action {
	// 	case "1":
	// 		tasks := GetTasks(db)
	// 		PrintTasks(tasks)
	// 	case "2":
	// 		fmt.Print("Введите название задачи: ")
	// 		title, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			ErrorHandler(err)
	// 		}
	// 		title = strings.TrimSpace(title)
	// 		fmt.Print("Введите статус задачи (true/false): ")
	// 		doneStr, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			ErrorHandler(err)
	// 		}
	// 		doneStr = strings.TrimSpace(doneStr)
	// 		done := (doneStr == "true")
	// 		AddTask(title, done, db)
	// 	case "3":
	// 		fmt.Print("Введите название задачи для редактирования: ")
	// 		title, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			ErrorHandler(err)
	// 		}
	// 		title = strings.TrimSpace(title)
	// 		fmt.Print("Введите новый статус задачи (true/false): ")
	// 		doneStr, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			ErrorHandler(err)
	// 		}
	// 		doneStr = strings.TrimSpace(doneStr)
	// 		done := (doneStr == "true")
	// 		EditTask(title, done, db)
	// 	case "4":
	// 		fmt.Print("Введите название задачи для удаления: ")
	// 		title, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			ErrorHandler(err)
	// 		}
	// 		title = strings.TrimSpace(title)
	// 		DeleteTask(title, db)
	// 	case "5":
	// 		fmt.Println("Выход из программы.")
	// 		return
	// 	}

	// }

}
