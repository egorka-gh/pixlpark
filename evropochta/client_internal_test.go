package evropochta

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_GetToken(t *testing.T) {

	cl, err := NewClient("https://api.eurotorg.by:10352/Json", "user", "pass", "servNum", "", nil)

	assert.Nil(t, err)
	err = cl.GetToken(context.TODO())
	assert.Nil(t, err)
	assert.True(t, cl.HasToken())
}
