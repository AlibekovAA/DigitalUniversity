package maxAPI

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"digitalUniversity/services"
)

const (
	teachersMessage         = "Добро пожаловать, преподаватель! 👨‍🏫\nФункционал для преподавателей находится в разработке."
	studentsMessage         = "Добро пожаловать, студент! 🎓\nФункционал для студентов находится в разработке."
	fileNotFoundMessage     = "Файл не найден. Отправьте CSV файл."
	sendStudentsFileMessage = "Отправьте файл со списком студентов (с расширением .csv)."
	sendTeachersFileMessage = "Отправьте файл с преподавателями (с расширением .csv)."
	sendScheduleFileMessage = "Отправьте файл с расписанием (с расширением .csv)."
	errorMessage            = "❌ Ошибка:\n\n%s\n\n"
	studentsSuccessMessage  = "✅ Студенты успешно загружены!"
	teachersSuccessMessage  = "✅ Преподаватели успешно загружены!"
	scheduleSuccessMessage  = "✅ Расписание успешно загружено!"
	defaultSuccessMessage   = "✅ Данные успешно загружены!"
	unknownMessageText      = "❓ Я не понимаю это сообщение.\n\nИспользуйте кнопки для взаимодействия с ботом."
	unknownMessageAdmin     = "❓ Я не понимаю это сообщение.\n\nИспользуйте кнопки меню для управления:"
	unknownMessageDefault   = "❓ Я не понимаю это сообщение.\n\nИспользуйте команду /start для начала работы с ботом."
	unknownMessageWithStart = "%s\n\nПожалуйста, используйте команду /start для начала работы."
	nextActionMessage       = "Выберите следующее действие:"
)

func (b *Bot) handleBotStarted(ctx context.Context, u *schemes.BotStartedUpdate) {
	sender := u.User

	if err := b.sendMessage(ctx, sender.UserId, welcomeMsg); err != nil {
		b.logger.Errorf("Failed to send start message: %v", err)
		return
	}

	userRole, err := b.getUserRole(sender.UserId)
	if err != nil {
		b.logger.Errorf("Failed to get role from db: %v", err)
		return
	}

	b.sendKeyboardByRole(ctx, sender.UserId, userRole)
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
		go b.processFileUpload(context.Background(), fileAttachments[0], uploadType, userID)
		return
	}

	b.mu.Lock()
	totalFiles := b.uploadCounter[userID]
	delete(b.uploadCounter, userID)
	delete(b.pendingUploads, userID)
	b.mu.Unlock()

	b.sendErrorAndResetUpload(ctx, userID, fmt.Sprintf("Отправлено %d файла(ов). Пожалуйста, отправьте только один CSV файл за раз.", totalFiles))
}

func (b *Bot) handleCallback(ctx context.Context, u *schemes.MessageCallbackUpdate) {
	sender := u.Callback.User
	chatID := u.GetChatID()

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
		if err := b.sendScheduleForDay(ctx, chatID, 1); err != nil {
			b.logger.Errorf("Failed to send schedule: %v", err)
		}
		return
	default:
		b.logger.Warnf("Unknown callback: %s", u.Callback.Payload)
		return
	}

	if err := b.sendMessage(ctx, sender.UserId, message); err != nil {
		b.logger.Errorf("Failed to send callback response: %v", err)
	}
}

func (b *Bot) sendKeyboard(ctx context.Context, keyboard *maxbot.Keyboard, userID int64, msg string) {
	_, err := b.MaxAPI.Messages.Send(ctx, maxbot.NewMessage().
		SetUser(userID).
		AddKeyboard(keyboard).
		SetText(msg))
	if err != nil && err.Error() != "" {
		b.logger.Errorf("Failed to send keyboard: %v", err)
	}
}

func (b *Bot) sendMessage(ctx context.Context, userID int64, text string) error {
	_, err := b.MaxAPI.Messages.Send(ctx, maxbot.NewMessage().
		SetUser(userID).
		SetText(text))
	if err != nil && err.Error() != "" {
		return err
	}
	return nil
}

func (b *Bot) getUserRole(userID int64) (string, error) {
	return b.userRepo.GetUserRole(userID)
}

func (b *Bot) sendKeyboardByRole(ctx context.Context, userID int64, role string) {
	var keyboard *maxbot.Keyboard
	var msg string

	switch role {
	case "admin":
		keyboard = GetAdminKeyboard(b.MaxAPI)
		msg = adminMsg
	case "teacher":
		keyboard = GetTeacherKeyboard(b.MaxAPI)
		msg = teachersMessage
	case "student":
		keyboard = GetStudentKeyboard(b.MaxAPI)
		msg = studentsMessage
	default:
		b.logger.Warnf("Unknown role: %q", role)
		return
	}

	_, err := b.MaxAPI.Messages.Send(ctx, maxbot.NewMessage().
		SetUser(userID).
		AddKeyboard(keyboard).
		SetText(msg))
	if err != nil && err.Error() != "" {
		b.logger.Errorf("Failed to send keyboard: %v", err)
	}
}

func (b *Bot) isMessageProcessed(messageID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.processedMessages[messageID]
}

func (b *Bot) markMessageProcessed(messageID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.processedMessages[messageID] = true
}

func (b *Bot) cleanupProcessedMessage(messageID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.processedMessages, messageID)
}

func (b *Bot) extractFileAttachments(attachments []interface{}) []*schemes.FileAttachment {
	fileAttachments := []*schemes.FileAttachment{}
	for _, att := range attachments {
		if fileAtt, ok := att.(*schemes.FileAttachment); ok {
			fileAttachments = append(fileAttachments, fileAtt)
		}
	}
	return fileAttachments
}

func (b *Bot) processFileUpload(ctx context.Context, fileAtt *schemes.FileAttachment, uploadType string, userID int64) {
	defer func() {
		b.mu.Lock()
		delete(b.pendingUploads, userID)
		delete(b.uploadCounter, userID)
		b.mu.Unlock()
	}()

	if err := b.downloadAndProcessFile(ctx, fileAtt, uploadType); err != nil {
		b.logger.Errorf("Failed to process file %s: %v", fileAtt.Filename, err)
		b.sendMessage(ctx, userID, fmt.Sprintf(errorMessage, err.Error()))

		userRole, _ := b.getUserRole(userID)
		b.sendKeyboardByRole(ctx, userID, userRole)
		return
	}

	b.sendSuccessMessage(ctx, userID, uploadType)
}

func (b *Bot) downloadAndProcessFile(ctx context.Context, fileAtt *schemes.FileAttachment, uploadType string) error {
	filePath, err := b.downloadFile(ctx, fileAtt)
	if err != nil {
		return err
	}
	defer os.Remove(filePath)

	if err := b.validateAndImportFile(filePath, uploadType); err != nil {
		return err
	}

	return nil
}

func (b *Bot) downloadFile(ctx context.Context, fileAtt *schemes.FileAttachment) (string, error) {
	fileURL := fileAtt.Payload.Url
	b.logger.Debugf("Downloading file: %s from %s", fileAtt.Filename, fileURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b.logger.Errorf("Bad HTTP status when downloading file: %s", resp.Status)
		return "", fmt.Errorf("failed to download file: status %s", resp.Status)
	}

	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	filePath := filepath.Join(tmpDir, fileAtt.Filename)

	if err := b.saveFile(filePath, resp.Body); err != nil {
		return "", err
	}

	b.logger.Infof("File saved to: %s", filePath)
	return filePath, nil
}

func (b *Bot) saveFile(filePath string, reader io.Reader) error {
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return err
}

func (b *Bot) validateAndImportFile(filePath, uploadType string) error {
	fileType := b.getFileType(uploadType)

	if err := services.ValidateCSVStructure(filePath, fileType); err != nil {
		return err
	}

	importer := services.NewCSVImporter(b.db)
	switch uploadType {
	case "students":
		return importer.ImportStudents(filePath)
	case "teachers":
		return importer.ImportTeachers(filePath)
	case "schedule":
		return importer.ImportSchedule(filePath)
	default:
		b.logger.Warnf("Unknown upload type: %s", uploadType)
		return fmt.Errorf("unknown upload type: %s", uploadType)
	}
}

func (b *Bot) getFileType(uploadType string) services.FileType {
	switch uploadType {
	case "students":
		return services.FileTypeStudents
	case "teachers":
		return services.FileTypeTeachers
	case "schedule":
		return services.FileTypeSchedule
	default:
		return ""
	}
}

func (b *Bot) sendErrorAndResetUpload(ctx context.Context, userID int64, errorMsg string) {
	b.sendMessage(ctx, userID, fmt.Sprintf(errorMessage, errorMsg))

	userRole, err := b.getUserRole(userID)
	if err != nil {
		b.logger.Errorf("Failed to get user role: %v", err)
		return
	}

	b.sendKeyboardByRole(ctx, userID, userRole)
	delete(b.pendingUploads, userID)
}

func (b *Bot) sendSuccessMessage(ctx context.Context, userID int64, uploadType string) {
	message := b.getSuccessMessage(uploadType)
	b.sendMessage(ctx, userID, message)
	b.sendKeyboard(ctx, GetAdminKeyboard(b.MaxAPI), userID, nextActionMessage)
}

func (b *Bot) getSuccessMessage(uploadType string) string {
	switch uploadType {
	case "students":
		return studentsSuccessMessage
	case "teachers":
		return teachersSuccessMessage
	case "schedule":
		return scheduleSuccessMessage
	default:
		return defaultSuccessMessage
	}
}

func (b *Bot) handleUnexpectedMessage(ctx context.Context, userID int64) {
	userRole, err := b.getUserRole(userID)
	if err != nil {
		b.logger.Errorf("Failed to get role from db: %v", err)
		b.sendMessage(ctx, userID, unknownMessageText)
		return
	}

	switch userRole {
	case "admin":
		b.sendMessage(ctx, userID, unknownMessageAdmin)
		b.sendKeyboard(ctx, GetAdminKeyboard(b.MaxAPI), userID, adminMsg)
	case "teacher", "student":
		b.sendMessage(ctx, userID, fmt.Sprintf(unknownMessageWithStart, unknownMessageText))
	default:
		b.sendMessage(ctx, userID, unknownMessageDefault)
	}

	delete(b.pendingUploads, userID)
}
