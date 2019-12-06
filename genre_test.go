package megasd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenre(t *testing.T) {
	assert.Equal(t, genre(21), genreOther)
}
