package entity

// HelloResponse contains the greeting returned by the hello endpoint.
type HelloResponse struct {
	Message string `json:"message" example:"hello"`
}
