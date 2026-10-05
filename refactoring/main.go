package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Order описывает структуру заказа
type Order struct {
	ID       int
	Customer string
	Products []string
	Total    float64
	Status   string
}

// ==========================================
// 1. АБСТРАКЦИИ (ИНТЕРФЕЙСЫ)
// ==========================================

// RepositoryWriter определяет контракт для записи в базу данных
type RepositoryWriter interface {
	Save(order *Order) error
}

// Notifier определяет контракт для отправки уведомлений
type Notifier interface {
	Send(customer string, message string) error
}

// DatabaseInitializer отвечает только за первичную настройку таблиц
type DatabaseInitializer interface {
	InitSchema() error
}

// ==========================================
// 2. РЕАЛИЗАЦИИ КОМПОНЕНТОВ
// ==========================================

// SQLiteRepository реализует работу с SQLite
type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) InitSchema() error {
	_, err := r.db.Exec(`
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer TEXT NOT NULL,
		products TEXT NOT NULL,
		total REAL NOT NULL,
		status TEXT NOT NULL
	)`)
	return err
}

func (r *SQLiteRepository) Save(order *Order) error {
	_, err := r.db.Exec(
		"INSERT INTO orders (customer, products, total, status) VALUES (?, ?, ?, ?)",
		order.Customer, fmt.Sprintf("%v", order.Products), order.Total, order.Status,
	)
	return err
}

// EmailSender — отправка уведомлений по почте
type EmailSender struct{}

func (e *EmailSender) Send(customer string, message string) error {
	fmt.Printf("[Email] Уведомление отправлено клиенту %s: %s\n", customer, message)
	return nil
}

// SMSSender — отправка уведомлений по SMS
type SMSSender struct{}

func (s *SMSSender) Send(customer string, message string) error {
	fmt.Printf("[SMS] Уведомление отправлено клиенту %s: %s\n", customer, message)
	return nil
}

// ==========================================
// 3. ОСНОВНОЙ СЕРВИС (БИЗНЕС-ЛОГИКА)
// ==========================================

type OrderService struct {
	repo     RepositoryWriter // Зависимость от интерфейса, а не от структуры
	notifier Notifier         // Зависимость от интерфейса, а не от структуры
}

// Конструктор принимает любые объекты, реализующие нужные интерфейсы
func NewOrderService(repo RepositoryWriter, notifier Notifier) *OrderService {
	return &OrderService{
		repo:     repo,
		notifier: notifier,
	}
}

func (s *OrderService) CreateOrder(customer string, products []string, total float64) error {
	order := &Order{
		Customer: customer,
		Products: products,
		Total:    total,
		Status:   "pending",
	}

	// Делегируем сохранение
	if err := s.repo.Save(order); err != nil {
		return fmt.Errorf("ошибка сохранения заказа: %w", err)
	}

	// Делегируем отправку
	msg := "Ваш заказ успешно создан!"
	if err := s.notifier.Send(customer, msg); err != nil {
		log.Printf("Не удалось отправить уведомление: %v", err)
	}

	return nil
}

// ==========================================
// 4. ТОЧКА ВХОДА И ДЕМОНСТРАЦИЯ
// ==========================================

func main() {
	db, err := sql.Open("sqlite3", "orders.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqliteRepo := NewSQLiteRepository(db)

	// Настройка базы данных
	if err := sqliteRepo.InitSchema(); err != nil {
		log.Fatal(err)
	}

	emailSender := &EmailSender{}
	smsSender := &SMSSender{}

	// Демонстрация 1: Работа с Email
	fmt.Println("--- Тест 1: Использование EmailSender ---")
	serviceWithEmail := NewOrderService(sqliteRepo, emailSender)
	_ = serviceWithEmail.CreateOrder("Иван", []string{"apple", "banana"}, 10.5)

	// Демонстрация 2: Работа с SMS (код самого OrderService остался нетронутым!)
	fmt.Println("\n--- Тест 2: Использование SMSSender ---")
	serviceWithSMS := NewOrderService(sqliteRepo, smsSender)
	_ = serviceWithSMS.CreateOrder("Пётр", []string{"orange"}, 5.2)
}
