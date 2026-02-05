package logstore_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/zetetos/simtezilo/app/logstore"
)

type LogStoreTestSuite struct {
	suite.Suite

	store *logstore.Store
}

func TestLogStoreTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LogStoreTestSuite))
}

func (suite *LogStoreTestSuite) SetupTest() {
	suite.store = logstore.New(10)
}

func (suite *LogStoreTestSuite) TestAddAndGetAll() {
	// Arrange
	store := logstore.New(5)

	// Act
	store.Add(logstore.LogEntry{
		"timestamp": "2024-01-01T10:00:00Z",
		"level":     "info",
		"message":   "Test message 1",
	})
	store.Add(logstore.LogEntry{
		"timestamp": "2024-01-01T10:00:01Z",
		"level":     "error",
		"message":   "Test message 2",
	})

	entries := store.GetAll()

	// Assert
	suite.Len(entries, 2, "Should have 2 entries")
}

func (suite *LogStoreTestSuite) TestCircularBuffer() {
	// Arrange
	store := logstore.New(3)

	// Act
	for i := 1; i <= 5; i++ {
		store.Add(logstore.LogEntry{
			"timestamp": "2024-01-01T10:00:00Z",
			"level":     "info",
			"message":   "Test message",
		})
	}

	entries := store.GetAll()
	count := store.Count()

	// Assert
	suite.Len(entries, 3, "Should only have 3 entries (max capacity)")
	suite.Equal(3, count, "Count should be 3")
}

func (suite *LogStoreTestSuite) TestGetStats() {
	// Arrange
	suite.store.Add(logstore.LogEntry{"level": "info", "message": "Test 1"})
	suite.store.Add(logstore.LogEntry{"level": "info", "message": "Test 2"})
	suite.store.Add(logstore.LogEntry{"level": "error", "message": "Test 3"})
	suite.store.Add(logstore.LogEntry{"level": "warn", "message": "Test 4"})

	// Act
	stats := suite.store.GetStats()

	// Assert
	suite.Equal(2, stats["info"], "Should have 2 info logs")
	suite.Equal(1, stats["error"], "Should have 1 error log")
	suite.Equal(1, stats["warn"], "Should have 1 warn log")
}

func (suite *LogStoreTestSuite) TestClear() {
	// Arrange
	suite.store.Add(logstore.LogEntry{"level": "info", "message": "Test 1"})
	suite.store.Add(logstore.LogEntry{"level": "info", "message": "Test 2"})

	suite.Equal(2, suite.store.Count(), "Should have 2 entries before clear")

	// Act
	suite.store.Clear()

	// Assert
	suite.Equal(0, suite.store.Count(), "Should have 0 entries after clear")
}

func (suite *LogStoreTestSuite) TestGetPageFirstPage() {
	// Arrange
	store := logstore.New(100)
	for i := 1; i <= 50; i++ {
		store.Add(logstore.LogEntry{
			"level":   "info",
			"message": "Test message",
			"index":   i,
		})
	}

	// Act
	page1 := store.GetPage(0, 10)

	// Assert
	suite.Len(page1, 10, "Should have 10 entries on page 1")
	// Should return newest first (50, 49, 48, ...)
	idx, ok := page1[0]["index"].(int)
	suite.True(ok, "Index should be an int")
	suite.Equal(50, idx, "First entry should have index 50")
}

func (suite *LogStoreTestSuite) TestGetPageSecondPage() {
	// Arrange
	store := logstore.New(100)
	for i := 1; i <= 50; i++ {
		store.Add(logstore.LogEntry{
			"level":   "info",
			"message": "Test message",
			"index":   i,
		})
	}

	// Act
	page2 := store.GetPage(10, 10)

	// Assert
	suite.Len(page2, 10, "Should have 10 entries on page 2")
	idx, ok := page2[0]["index"].(int)
	suite.True(ok, "Index should be an int")
	suite.Equal(40, idx, "First entry of page 2 should have index 40")
}

func (suite *LogStoreTestSuite) TestGetPageBeyondAvailableEntries() {
	// Arrange
	store := logstore.New(100)
	for i := 1; i <= 50; i++ {
		store.Add(logstore.LogEntry{
			"level":   "info",
			"message": "Test message",
			"index":   i,
		})
	}

	// Act
	page6 := store.GetPage(50, 10)

	// Assert
	suite.Empty(page6, "Should have 0 entries beyond available data")
}

func (suite *LogStoreTestSuite) TestGetPagePartialPage() {
	// Arrange
	store := logstore.New(100)
	for i := 1; i <= 50; i++ {
		store.Add(logstore.LogEntry{
			"level":   "info",
			"message": "Test message",
			"index":   i,
		})
	}

	// Act
	page5 := store.GetPage(45, 10)

	// Assert
	suite.Len(page5, 5, "Should have 5 entries on partial page")
}
