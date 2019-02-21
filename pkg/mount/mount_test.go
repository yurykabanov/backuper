package mount

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_AllocateDeallocate(t *testing.T) {
	m := New("/tmp")

	dir, err := m.Allocate()

	assert.Nil(t, err)
	assert.DirExists(t, dir)
	assert.True(t, strings.HasPrefix(dir, "/tmp/"))

	err = m.Deallocate(dir)

	assert.Nil(t, err)

	_, err = os.Stat(dir)
	_, ok := err.(*os.PathError)

	assert.True(t, ok)
}

func TestManager_Allocate_Error(t *testing.T) {
	m := New("/bad_directory")

	dir, err := m.Allocate()

	assert.NotNil(t, err)
	assert.Equal(t, "", dir)
}
