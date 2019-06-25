package service

import "context"

// PPService describes the pixlpark service.
type PPService interface {
	CountOrders(ctx context.Context, statuses []string) (int, error)
}
