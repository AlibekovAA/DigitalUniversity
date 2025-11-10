package maxAPI

import (
	"context"
	"fmt"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

const (
	welcomeTeacherMsg = "Добро пожаловать, преподаватель! 👨‍🏫"
	welcomeStudentMsg = "Добро пожаловать, студент! 🎓"
	welcomeAdminMsg   = "Добро пожаловать, администратор! 👨‍💼"

	mainMenuAdminMsg   = "Главное меню администратора:"
	mainMenuTeacherMsg = "Главное меню преподавателя:"
	mainMenuStudentMsg = "Главное меню студента:"

	unknownMessage        = "❓ Я не понимаю это сообщение."
	unknownMessageDefault = "❓ Я не понимаю это сообщение.\n\nОбратитесь к администратору для получения доступа."
	retryActionMessage    = "Попробуйте снова или выберите другое действие:"

	fileNotFoundMessage     = "Файл не найден. Отправьте CSV файл."
	multipleFilesMessage    = "Отправлено %d файла(ов). Пожалуйста, отправьте только один CSV файл за раз."
	sendStudentsFileMessage = "Отправьте файл со списком студентов (с расширением .csv)."
	sendTeachersFileMessage = "Отправьте файл с преподавателями (с расширением .csv)."
	sendScheduleFileMessage = "Отправьте файл с расписанием (с расширением .csv)."
	errorMessage            = "❌ Ошибка:\n\n%s\n\n"
	studentsSuccessMessage  = "✅ Студенты успешно загружены!"
	teachersSuccessMessage  = "✅ Преподаватели успешно загружены!"
	scheduleSuccessMessage  = "✅ Расписание успешно загружено!"
	defaultSuccessMessage   = "✅ Данные успешно загружены!"
	nextActionMessage       = "Выберите следующее действие:"
)

func (b *Bot) handleBotStarted(ctx context.Context, u *schemes.BotStartedUpdate) {
	sender := u.User

	userRole, err := b.getUserRole(sender.UserId)
	if err != nil {
		b.logger.Errorf("Failed to get role from db: %v", err)
		return
	}

	b.sendWelcomeWithKeyboard(ctx, sender.UserId, userRole)
}

func (b *Bot) handleMessageCreated(ctx context.Context, u *schemes.MessageCreatedUpdate) {
	userID := u.Message.Sender.UserId
	messageID := u.Message.Body.Mid

	if b.isMessageProcessed(messageID) {
		b.logger.Debugf("Message %s already processed, skipping", messageID)
		return
	}

	b.markMessageProcessed(messageID)
	defer b.cleanupProcessedMessage(messageID)

	attachments := u.Message.Body.Attachments
	messageText := u.Message.Body.Text

	if len(attachments) == 0 && messageText != "" {
		b.handleUnexpectedMessage(ctx, userID)
		return
	}

	if len(attachments) == 0 {
		return
	}

	uploadType := b.pendingUploads[userID]
	if uploadType == "" {
		b.logger.Warnf("No pending upload for user %d", userID)
		b.handleUnexpectedMessage(ctx, userID)
		return
	}

	fileAttachments := b.extractFileAttachments(attachments)

	if len(fileAttachments) == 0 {
		b.sendErrorAndResetUpload(ctx, userID, fileNotFoundMessage)
		return
	}

	b.mu.Lock()
	b.uploadCounter[userID]++
	count := b.uploadCounter[userID]
	b.mu.Unlock()

	if count == 1 {
		go func() {
			time.Sleep(500 * time.Millisecond)

			b.mu.Lock()
			totalFiles := b.uploadCounter[userID]
			delete(b.uploadCounter, userID)
			delete(b.pendingUploads, userID)
			b.mu.Unlock()

			if totalFiles > 1 {
				b.sendErrorAndResetUpload(ctx, userID, fmt.Sprintf(multipleFilesMessage, totalFiles))
				return
			}

			if err := b.downloadAndProcessFile(ctx, fileAttachments[0], uploadType); err != nil {
				b.logger.Errorf("Failed to process file %s: %v", fileAttachments[0].Filename, err)
				b.sendMessage(ctx, userID, fmt.Sprintf(errorMessage, err.Error()))
				b.sendKeyboardAfterError(ctx, userID)
				return
			}

			b.sendSuccessMessage(ctx, userID, uploadType)
		}()
	}
}

func (b *Bot) handleCallback(ctx context.Context, u *schemes.MessageCallbackUpdate) {
	sender := u.Callback.User
	userID := sender.UserId
	callbackID := u.Callback.CallbackID

	messageID := ""
	if u.Message != nil {
		messageID = u.Message.Body.Mid
	}

	if messageID == "" {
		b.mu.Lock()
		messageID = b.lastMessageID[userID]
		b.mu.Unlock()
	}

	b.logger.Debugf("Callback received: payload=%s, callbackID=%s, userID=%d, messageID=%s",
		u.Callback.Payload, callbackID, userID, messageID)

	var message string

	switch u.Callback.Payload {
	case "uploadStudents":
		message = sendStudentsFileMessage
		b.pendingUploads[sender.UserId] = "students"
	case "uploadTeachers":
		message = sendTeachersFileMessage
		b.pendingUploads[sender.UserId] = "teachers"
	case "uploadSchedule":
		message = sendScheduleFileMessage
		b.pendingUploads[sender.UserId] = "schedule"
	case "showSchedule":
		currentWeekday := int16(time.Now().Weekday())
		if currentWeekday == 0 {
			currentWeekday = 7
		}
		if err := b.sendScheduleForDay(ctx, userID, callbackID, currentWeekday); err != nil {
			b.logger.Errorf("Failed to send schedule: %v", err)
		}
		return
	case "markGrade":
		if err := b.handleMarkGradeStart(ctx, userID, callbackID); err != nil {
			b.logger.Errorf("Failed to start grade marking: %v", err)
		}
		return
	case "showScore":
		if err := b.handleShowGradesStart(ctx, userID, callbackID); err != nil {
			b.logger.Errorf("Failed to show grades: %v", err)
		}
		return
	case "backToMenu":
		if err := b.handleBackToMenu(ctx, userID, callbackID); err != nil {
			b.logger.Errorf("Failed to return to menu: %v", err)
		}
		return
	default:
		if strings.HasPrefix(u.Callback.Payload, "sch_day_") {
			var day int16
			fmt.Sscanf(u.Callback.Payload, "sch_day_%d", &day)

			b.logger.Debugf("Processing schedule navigation: day=%d, callbackID=%s", day, callbackID)

			if err := b.answerScheduleCallback(ctx, userID, callbackID, day); err != nil {
				b.logger.Errorf("Failed to answer callback: %v", err)
			}
			return
		}

		if strings.HasPrefix(u.Callback.Payload, "grade_") {
			if err := b.handleGradeCallback(ctx, userID, callbackID, u.Callback.Payload); err != nil {
				b.logger.Errorf("Failed to handle grade callback: %v", err)
			}
			return
		}

		if strings.HasPrefix(u.Callback.Payload, "show_grades_") {
			if err := b.handleShowGradesCallback(ctx, userID, callbackID, u.Callback.Payload); err != nil {
				b.logger.Errorf("Failed to handle show grades callback: %v", err)
			}
			return
		}

		b.logger.Warnf("Unknown callback: %s", u.Callback.Payload)
		return
	}

	if err := b.sendMessage(ctx, sender.UserId, message); err != nil {
		b.logger.Errorf("Failed to send callback response: %v", err)
	}
}

func (b *Bot) handleBackToMenu(ctx context.Context, userID int64, callbackID string) error {
	userRole, err := b.getUserRole(userID)
	if err != nil {
		b.logger.Errorf("Failed to get user role: %v", err)
		return err
	}

	var keyboard *maxbot.Keyboard
	var menuText string

	switch userRole {
	case "admin":
		keyboard = GetAdminKeyboard(b.MaxAPI)
		menuText = mainMenuAdminMsg
	case "teacher":
		keyboard = GetTeacherKeyboard(b.MaxAPI)
		menuText = mainMenuTeacherMsg
	case "student":
		keyboard = GetStudentKeyboard(b.MaxAPI)
		menuText = mainMenuStudentMsg
	default:
		b.logger.Warnf("Unknown role: %s", userRole)
		return fmt.Errorf("unknown role: %s", userRole)
	}

	messageBody := &schemes.NewMessageBody{
		Text:        menuText,
		Attachments: []interface{}{schemes.NewInlineKeyboardAttachmentRequest(keyboard.Build())},
	}

	answer := &schemes.CallbackAnswer{Message: messageBody}
	_, err = b.MaxAPI.Messages.AnswerOnCallback(ctx, callbackID, answer)
	if err != nil && err.Error() != "" {
		b.logger.Errorf("Failed to answer callback: %v", err)
		return err
	}

	b.logger.Infof("User %d returned to main menu (role: %s)", userID, userRole)
	return nil
}

func (b *Bot) handleUnexpectedMessage(ctx context.Context, userID int64) {
	userRole, err := b.getUserRole(userID)
	if err != nil {
		b.logger.Errorf("Failed to get role from db: %v", err)
		b.sendMessage(ctx, userID, unknownMessageDefault)
		return
	}

	switch userRole {
	case "admin":
		b.sendKeyboard(ctx, GetAdminKeyboard(b.MaxAPI), userID, unknownMessage)
	case "teacher":
		b.sendKeyboard(ctx, GetTeacherKeyboard(b.MaxAPI), userID, unknownMessage)
	case "student":
		b.sendKeyboard(ctx, GetStudentKeyboard(b.MaxAPI), userID, unknownMessage)
	default:
		b.sendMessage(ctx, userID, unknownMessageDefault)
	}

	delete(b.pendingUploads, userID)
}
