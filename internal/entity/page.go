package entity

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Page is shared by list requests and results. Normalize before Validate.
type Page struct {
	Page     int `form:"page" json:"page" binding:"omitempty,min=1" minimum:"0" default:"1"`
	PageSize int `form:"page_size" json:"page_size" binding:"omitempty,min=1,max=100" minimum:"0" maximum:"100" default:"20"`
}

func (p *Page) Normalize() {
	if p.Page == 0 {
		p.Page = DefaultPage
	}
	if p.PageSize == 0 {
		p.PageSize = DefaultPageSize
	}
}

// Validate rejects invalid ranges and an offset that would overflow int.
func (p Page) Validate() error {
	maxInt := int(^uint(0) >> 1)
	if p.Page < 1 || p.PageSize < 1 || p.PageSize > MaxPageSize || p.Page-1 > maxInt/p.PageSize {
		return ErrInvalidArgument
	}
	return nil
}

// Offset requires normalized, validated pagination.
func (p Page) Offset() int { return (p.Page - 1) * p.PageSize }
func (p Page) Limit() int  { return p.PageSize }

// PageList contains the filtered total before pagination. Use a non-nil List for [].
type PageList[T any] struct {
	Page
	Total int64 `json:"total"`
	List  []T   `json:"list"`
}
