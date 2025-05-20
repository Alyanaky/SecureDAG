package integration

import (
    "net/http"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestLoadBalancing(t *testing.T) {
    // Тест балансировки нагрузки
    resp := PUT("/objects/load-test", "large-data")
    assert.Equal(t, http.StatusOK, resp.Code)
    
    // Проверка распределения по узлам
    nodes := GET("/admin/nodes")
    assert.True(t, len(nodes) >= 3, "Должно быть минимум 3 узла")
}

func TestQuotaManagement(t *testing.T) {
    // Установка квоты
    req := `{"quota": 1024}`
    resp := PUT("/admin/users/user1/quota", req)
    assert.Equal(t, http.StatusOK, resp.Code)
    
    // Проверка квоты
    resp = GET("/admin/users/user1")
    assert.Equal(t, int64(1024), resp.Body.Quota)
}
