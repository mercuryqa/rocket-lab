package testutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AssertGRPCStatus проверяет, что ошибка является gRPC ошибкой с указанным кодом.
func AssertGRPCStatus(t *testing.T, err error, expectedCode codes.Code) {
	t.Helper()

	if expectedCode == codes.OK {
		assert.NoError(t, err, "ожидалось отсутствие ошибки")
		return
	}

	require.Error(t, err, "ожидалась ошибка")

	st, ok := status.FromError(err)
	assert.True(t, ok, "ошибка должна быть gRPC статусом")
	assert.Equal(t, expectedCode, st.Code(), "неверный gRPC код: %s", st.Message())
}

// AssertHTTPStatus проверяет HTTP статус код ответа.
func AssertHTTPStatus(t *testing.T, resp *http.Response, expectedCode int) {
	t.Helper()
	assert.Equal(t, expectedCode, resp.StatusCode, "неверный HTTP статус код")
}
