package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseChatToTransaction(t *testing.T) {
	t.Run("Valid Transaction Format", func(t *testing.T) {
		desc, amount, ok := ParseChatToTransaction("Nasi Padang 25000")
		assert.True(t, ok)
		assert.Equal(t, "Nasi Padang", desc)
		assert.Equal(t, float64(25000), amount)
	})

	t.Run("Valid Format with Extra Spaces", func(t *testing.T) {
		desc, amount, ok := ParseChatToTransaction("Bayar Listrik Bulan Ini   500000")
		assert.True(t, ok)
		assert.Equal(t, "Bayar Listrik Bulan Ini", desc)
		assert.Equal(t, float64(500000), amount)
	})

	t.Run("Invalid Format Without Number", func(t *testing.T) {
		_, _, ok := ParseChatToTransaction("Beli Makan Doang")
		assert.False(t, ok)
	})
}
