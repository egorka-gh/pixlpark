package photocycle

// Repository describes the persistence on dto
type Repository interface {
	//ListSource(ctx context.Context, source string) ([]Source, error)
	Close()
}
