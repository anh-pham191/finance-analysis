package domain

type CategoryKind string

const (
	CategoryKindIncome   CategoryKind = "income"
	CategoryKindExpense  CategoryKind = "expense"
	CategoryKindTransfer CategoryKind = "transfer"
)

type Category struct {
	ID       int64
	Name     string
	ParentID *int64
	Kind     CategoryKind
}
