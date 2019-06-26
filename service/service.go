package service

import "context"

//http://api.pixlpark.com/

//APIVersion implemented api version
const APIVersion = "1.0"

// PPService describes the pixlpark service.
type PPService interface {
	CountOrders(ctx context.Context, statuses []string) (int, error)
}
