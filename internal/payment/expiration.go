package payment

import (
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func NewWithExpiration(store Store, gateways gateway.Registry, productName string, expiration time.Duration) *Service {
	s := New(store, gateways, productName)
	if expiration > 0 {
		s.expiration = expiration
	}
	return s
}
