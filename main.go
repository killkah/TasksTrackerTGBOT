package main

import (
	"database/sql"
	"fmt"
	"log"

	"net/http"
	"net/url"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite" // Импорт Pure-Go драйвера
)

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
		task_id INTEGER NOT NULL,
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
	// Константы кнопок
	const (
		BtnTasks  = "📋 Мои задачи"
		BtnAdd    = "➕ Добавить задачу"
		BtnDelete = "❌ Удалить задачу"
	)
	// Константы шагов/состояний
	const (
		StateNone         = ""
		StateAwaitingAdd  = "awaiting_add"
		StateAwaitingEdit = "awaiting_edit"
		StateAwaitingDel  = "awaiting_del"
	)

	// Карта для хранения состояния каждого пользователя
	var userStates = make(map[int64]string)
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
					"/edit ID - Изменить статус задачи\n"+
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

		// 1. ПРОВЕРКА СОСТОЯНИЯ: Ждем ли мы текстовый ввод от этого пользователя?
		currentState := userStates[userID]

		if currentState != StateNone {
			// Пользователь прислал текст в ответ на ожидание!
			switch currentState {

			case StateAwaitingAdd:
				// Вызываем функцию добавления задачи, передавая 'text' как название
				HandleAddTask(text, bot, chatID, userID, db)
				// Сбрасываем состояние
				userStates[userID] = StateNone

			case StateAwaitingDel:
				// Вызываем функцию удаления
				HandleDeleteTask(text, bot, chatID, userID, db)
				userStates[userID] = StateNone

			case StateAwaitingEdit:
				// Вызываем функцию редактирования
				HandleEditTask(text, bot, chatID, userID, db)
				userStates[userID] = StateNone
			}

			// Завершаем обработку этого сообщения, чтобы не идти в стандартный switch
			continue
		}

		// 2. СТАНДАРТНЫЙ SWITCH (когда нет активного состояния)
		switch {
		case text == "/start":
			HandleStart(bot, chatID, userID, db)
		case text == "/tasks" || text == BtnTasks:
			HandlePrintTasks(bot, chatID, userID, db)
		case text == "/add" || text == BtnAdd:
			// Ставим пользователя в состояние ожидания текста
			userStates[userID] = StateAwaitingAdd
			bot.Send(tgbotapi.NewMessage(chatID, "📝 Отправьте текст для новой задачи:"))

		case text == "/delete" || text == BtnDelete:
			userStates[userID] = StateAwaitingDel
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Введите ID задачи, которую нужно удалить:"))

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
}
