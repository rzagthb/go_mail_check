package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

type Config struct {
	SMTPServer string
	SMTPPort   string
	IMAPServer string
	IMAPPort   string
	Email      string
	Password   string
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	cfg := Config{}

	fmt.Println("=== Утилита проверки почтового тракта ===")
	fmt.Println("Выберите почтовый сервис:")
	fmt.Println("1. google.com (Gmail)")
	fmt.Println("2. yandex.ru")
	fmt.Println("3. mail.ru")
	fmt.Println("4. other (Свой сервер)")
	fmt.Print("Ваш выбор (1-4): ")

	choice := readInput(reader)

	switch choice {
	case "1":
		cfg.SMTPServer, cfg.SMTPPort = "smtp.gmail.com", "587"
		cfg.IMAPServer, cfg.IMAPPort = "imap.gmail.com", "993"
	case "2":
		cfg.SMTPServer, cfg.SMTPPort = "smtp.yandex.ru", "587"
		cfg.IMAPServer, cfg.IMAPPort = "imap.yandex.ru", "993"
	case "3":
		cfg.SMTPServer, cfg.SMTPPort = "smtp.mail.ru", "587"
		cfg.IMAPServer, cfg.IMAPPort = "imap.mail.ru", "993"
	case "4":
		fmt.Print("\nВведите адрес SMTP сервера: ")
		cfg.SMTPServer = readInput(reader)
		fmt.Print("Введите порт SMTP (обычно 587): ")
		cfg.SMTPPort = readInput(reader)
		fmt.Print("Введите адрес IMAP сервера: ")
		cfg.IMAPServer = readInput(reader)
		fmt.Print("Введите порт IMAP (обычно 993): ")
		cfg.IMAPPort = readInput(reader)
	default:
		fmt.Println("❌ Неверный выбор. Выход.")
		return
	}

	for {
		fmt.Print("\nВведите ваш email (login): ")
		cfg.Email = readInput(reader)

		_, err := mail.ParseAddress(cfg.Email)
		if err != nil {
			fmt.Print("⚠️  Внимание: формат email нестандартный. Продолжить? (y/n): ")
			confirm := strings.ToLower(readInput(reader))
			if confirm == "y" || confirm == "yes" || confirm == "да" || confirm == "д" {
				break
			}
			continue
		}
		break
	}

	fmt.Print("Введите пароль приложения: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("\n❌ Ошибка при чтении пароля: %v", err)
	}
	cfg.Password = strings.TrimSpace(string(bytePassword))
	fmt.Println()

	subject := fmt.Sprintf("Ping_Mail_Tract_%v", time.Now().Unix())

	fmt.Printf("\n[1/3] Отправка тестового письма (SMTP)... ")
	if err := sendCustomEmail(cfg, subject); err != nil {
		log.Fatalf("\n❌ Ошибка SMTP: %v\n", err)
	}
	fmt.Println("ОК")

	// Анимированный прогресс-бар на 15 секунд
	waitTime := 15
	fmt.Print("[2/3] Ожидание доставки: ")
	for i := 0; i <= waitTime; i++ {
		// Строим полосу загрузки
		bar := strings.Repeat("█", i) + strings.Repeat("░", waitTime-i)
		// Символ \r возвращает каретку в начало строки, чтобы перезаписывать текст
		fmt.Printf("\r[2/3] Ожидание доставки: [%s] %d/%d сек", bar, i, waitTime)
		if i < waitTime {
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Println(" -> ОК")

	fmt.Print("[3/3] Проверка входящих (IMAP)... ")
	if err := checkCustomIMAP(cfg, subject); err != nil {
		log.Fatalf("\n❌ Ошибка IMAP: %v\n", err)
	}
	fmt.Println("✅ УСПЕХ! Письмо найдено.")
}

func readInput(r *bufio.Reader) string {
	input, _ := r.ReadString('\n')
	return strings.TrimSpace(input)
}

func sendCustomEmail(cfg Config, subject string) error {
	dateStr := time.Now().Format(time.RFC1123Z)

	msg := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"MIME-version: 1.0;\r\n"+
		"Content-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"+
		"Тестовое сообщение для проверки тракта.\r\n", cfg.Email, cfg.Email, subject, dateStr))

	addr := cfg.SMTPServer + ":" + cfg.SMTPPort
	
	// Попытка 1: стандартный PLAIN (работает для Google, Yandex, Mail.ru)
	authPlain := smtp.PlainAuth("", cfg.Email, cfg.Password, cfg.SMTPServer)
	err := smtp.SendMail(addr, authPlain, cfg.Email, []string{cfg.Email}, msg)

	// Если сервер вернул ошибку 504 (механизм не поддерживается), пробуем LOGIN
	if err != nil && strings.Contains(err.Error(), "504") {
		fmt.Print("[Сервер не поддерживает PLAIN, пробуем LOGIN...] ")
		authLogin := LoginAuth(cfg.Email, cfg.Password)
		err = smtp.SendMail(addr, authLogin, cfg.Email, []string{cfg.Email}, msg)
	}

	return err
}

func checkCustomIMAP(cfg Config, subject string) error {
	addr := cfg.IMAPServer + ":" + cfg.IMAPPort
	
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: false})
	if err != nil {
		return fmt.Errorf("ошибка TLS: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	if _, err := reader.ReadString('\n'); err != nil {
		return err
	}

	fmt.Fprintf(conn, "A1 LOGIN \"%s\" \"%s\"\r\n", cfg.Email, cfg.Password)
	if err := waitForIMAPResponse(reader, "A1"); err != nil {
		return fmt.Errorf("ошибка авторизации: %v", err)
	}

	fmt.Fprintf(conn, "A2 SELECT INBOX\r\n")
	if err := waitForIMAPResponse(reader, "A2"); err != nil {
		return fmt.Errorf("ошибка открытия INBOX: %v", err)
	}

	fmt.Fprintf(conn, "A3 SEARCH SUBJECT \"%s\"\r\n", subject)
	
	found := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("ошибка чтения ответа SEARCH: %v", err)
		}
		
		if strings.HasPrefix(line, "* SEARCH") && len(strings.TrimSpace(line)) > 8 {
			found = true
		}
		
		if strings.HasPrefix(line, "A3 OK") {
			break
		} else if strings.HasPrefix(line, "A3 NO") || strings.HasPrefix(line, "A3 BAD") {
			return fmt.Errorf("сервер отклонил поиск: %s", line)
		}
	}

	fmt.Fprintf(conn, "A4 LOGOUT\r\n")

	if !found {
		return fmt.Errorf("письмо с темой '%s' не найдено на сервере", subject)
	}
	return nil
}

func waitForIMAPResponse(reader *bufio.Reader, tag string) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" OK") {
			return nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return fmt.Errorf(strings.TrimSpace(line))
		}
	}
}
// --- Кастомный обработчик для AUTH LOGIN ---

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// Начинаем авторизацию, не передавая данные сразу
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		prompt := strings.ToLower(string(fromServer))
		// Сервер запрашивает логин
		if strings.Contains(prompt, "username") {
			return []byte(a.username), nil
		}
		// Сервер запрашивает пароль
		if strings.Contains(prompt, "password") {
			return []byte(a.password), nil
		}
		return nil, fmt.Errorf("неизвестный ответ сервера при LOGIN: %s", string(fromServer))
	}
	return nil, nil
}
