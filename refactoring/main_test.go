package main

import (
	"testing"
)

// ==========================================
// СОЗДАЕМ «МОКИ» (MOCKS) ДЛЯ ТЕСТИРОВАНИЯ
// ==========================================

// MockRepositoryWriter имитирует базу данных без реальной записи на диск
type MockRepositoryWriter struct {
	SaveCalled bool // Флаг, проверяющий, что метод действительно вызвался
}

func (m *MockRepositoryWriter) Save(order *Order) error {
	m.SaveCalled = true
	return nil // Имитируем успешное сохранение без ошибок БД
}

// MockNotifier имитирует отправку уведомлений без реального вывода на экран
type MockNotifier struct {
	SendCalled bool // Флаг, проверяющий отправку сообщения
}

func (m *MockNotifier) Send(customer string, message string) error {
	m.SendCalled = true
	return nil // Имитируем успешную отправку
}

// ==========================================
// САМИ ТЕСТЫ (UNIT TESTS)
// ==========================================

func TestCreateOrder_Success(t *testing.T) {
	// 1. Создаем изолированные моки вместо реальной базы и почты
	mockRepo := &MockRepositoryWriter{}
	mockNotifier := &MockNotifier{}

	// 2. Внедряем эти моки в наш OrderService
	service := NewOrderService(mockRepo, mockNotifier)

	// 3. Вызываем тестируемый метод
	err := service.CreateOrder("Тестовый Клиент", []string{"item1"}, 100.0)

	// 4. Проверяем утверждения (Assertions)
	if err != nil {
		t.Fatalf("Ожидался успешный заказ, получена ошибка: %v", err)
	}

	if !mockRepo.SaveCalled {
		t.Error("Ошибка: метод Save у репозитория не был вызван!")
	}

	if !mockNotifier.SendCalled {
		t.Error("Ошибка: метод Send у нотификатора не был вызван!")
	}
}